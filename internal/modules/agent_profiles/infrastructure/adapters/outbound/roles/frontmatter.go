// Package roles reads and writes the agent roles, one `<role>.md` file per role, with a minimal
// YAML frontmatter.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package roles

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routingjson"
)

const frontmatterDelimiter = "---"

// quoteStartingCharacters are the characters that force a scalar to be quoted when it starts with
// them, because YAML would read them as indicators.
const quoteStartingCharacters = "\"'#&*!|>%@`[]{},-?"

// frontmatter holds the scalar keys the parser understands. An empty model or thinking means the
// key was absent: the Rust parser ignored empty values for those two keys.
type frontmatter struct {
	name        string
	description string
	model       string
	thinking    string
}

// parseFrontmatter reads the scalar first-level keys between the two `---` lines. Lists, nested
// values and the body are ignored. Content without an opening delimiter yields no keys.
func parseFrontmatter(content string) frontmatter {
	var parsed frontmatter
	lines := strings.Split(content, "\n")
	if strings.TrimSpace(lines[0]) != frontmatterDelimiter {
		return parsed
	}
	for _, line := range lines[1:] {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == frontmatterDelimiter {
			break
		}
		if startsWithSpace(line) {
			continue
		}
		key, rawValue, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value := unquote(rawValue)
		switch strings.TrimSpace(key) {
		case "name":
			parsed.name = value
		case "description":
			parsed.description = value
		case "model":
			if value != "" {
				parsed.model = value
			}
		case "thinking":
			if value != "" {
				parsed.thinking = value
			}
		}
	}
	return parsed
}

func startsWithSpace(line string) bool {
	first, _ := utf8.DecodeRuneInString(line)
	return unicode.IsSpace(first)
}

// unquote removes one level of double or single quotes. Double quotes are decoded as a JSON string
// and fall back to the raw text when that fails; single quotes double their inner apostrophes.
func unquote(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		var decoded string
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
		return trimmed[1 : len(trimmed)-1]
	}
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") {
		return strings.ReplaceAll(trimmed[1:len(trimmed)-1], "''", "'")
	}
	return trimmed
}

// roleFromFile builds a role from a file. The name in the frontmatter wins when it is valid; the
// file stem is the fallback. A file with neither valid name is not a role.
func roleFromFile(stem string, content string) (models.AgentRole, bool) {
	parsed := parseFrontmatter(content)
	name, err := models.ParseRoleName(parsed.name)
	if err != nil {
		name, err = models.ParseRoleName(stem)
		if err != nil {
			return models.AgentRole{}, false
		}
	}
	return models.AgentRole{
		Name:        name,
		Description: parsed.description,
		Model:       optionalText(parsed.model),
		Thinking:    optionalText(parsed.thinking),
	}, true
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// renderRole writes the file content: frontmatter with name, description and the optional model
// and thinking, followed by the prompt. Newlines in single-line values are refused.
func renderRole(role models.NewAgentRole) (string, error) {
	description, err := singleLineValue("la descripción", role.Description)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(frontmatterDelimiter + "\n")
	out.WriteString("name: " + role.Name.String() + "\n")
	out.WriteString("description: " + description + "\n")
	if role.Model != nil {
		model, err := singleLineValue("el modelo", *role.Model)
		if err != nil {
			return "", err
		}
		out.WriteString("model: " + model + "\n")
	}
	if role.Thinking != nil {
		thinking, err := singleLineValue("el razonamiento", *role.Thinking)
		if err != nil {
			return "", err
		}
		out.WriteString("thinking: " + thinking + "\n")
	}
	out.WriteString(frontmatterDelimiter + "\n")
	prompt := strings.TrimRightFunc(role.Prompt, unicode.IsSpace)
	if prompt != "" {
		out.WriteString("\n" + prompt + "\n")
	}
	return out.String(), nil
}

func singleLineValue(label string, value string) (string, error) {
	if strings.ContainsAny(value, "\n\r") {
		return "", agenterrors.Store(label + " no puede contener saltos de línea")
	}
	if needsQuotes(value) {
		return encodeString(value)
	}
	return value, nil
}

// needsQuotes reports whether a plain YAML scalar would be misread: empty, padded with spaces,
// containing a mapping or comment marker, ending with a colon, or starting with an indicator.
func needsQuotes(value string) bool {
	if value == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(value)
	return value != strings.TrimSpace(value) ||
		strings.Contains(value, ": ") ||
		strings.Contains(value, " #") ||
		strings.HasSuffix(value, ":") ||
		strings.ContainsRune(quoteStartingCharacters, first)
}

// encodeString quotes a value as a JSON string, without HTML escaping.
func encodeString(value string) (string, error) {
	quoted, err := routingjson.Compact(value)
	if err != nil {
		return "", agenterrors.Store(err.Error())
	}
	return quoted, nil
}
