//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

// maxErrorBodyRunes bounds the provider text kept in an error message.
const maxErrorBodyRunes = 300

// encodeComponent percent-encodes one path component (a project or a branch): everything except the
// unreserved characters, so the slash of `group/project` becomes %2F.
func encodeComponent(value string) string {
	var builder strings.Builder
	for index := range len(value) {
		current := value[index]
		if isUnreserved(current) {
			builder.WriteByte(current)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", current)
	}
	return builder.String()
}

func isUnreserved(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') || value == '-' || value == '_' || value == '.' || value == '~'
}

// withQuery appends the pairs in the order given (the providers do not care, but the request
// targets stay the ones the Rust version sent).
func withQuery(path string, pairs ...[2]string) string {
	if len(pairs) == 0 {
		return path
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, url.QueryEscape(pair[0])+"="+url.QueryEscape(pair[1]))
	}
	return path + "?" + strings.Join(parts, "&")
}

// newRequest builds one request. Errors never carry the URL or the headers, which hold the token.
func newRequest(ctx context.Context, method string, target string, headers map[string]string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, codeerrors.Network(networkMessage(err))
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// send performs the request. A 2xx answer returns its body; a redirect is reported as Redirected and
// never followed; any other status is an Api error carrying the message of the provider.
func send(client *http.Client, provider models.HostingProvider, request *http.Request) ([]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, codeerrors.Network(networkMessage(err))
	}
	defer func() { _ = response.Body.Close() }()
	body, readErr := io.ReadAll(response.Body)
	status := response.StatusCode
	switch {
	case status >= 200 && status < 300:
		if readErr != nil {
			return nil, codeerrors.Network(networkMessage(readErr))
		}
		return body, nil
	case status >= 300 && status < 400:
		return nil, codeerrors.Redirected(provider, status)
	default:
		return nil, codeerrors.API(provider, status, errorMessage(string(body), status))
	}
}

// decodeJSON reads a provider answer. A malformed answer is a network-level failure, as in the
// Rust version.
func decodeJSON(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err != nil {
		return codeerrors.Network(err.Error())
	}
	return nil
}

// networkMessage drops the URL that net/http puts in its errors, so nothing else leaks with it.
func networkMessage(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

// errorMessage extracts the human message of a provider error body. GitLab answers with `message`
// (text, list or object) or `error`; GitHub with `message` plus `errors`, where the real reason of a
// 422 lives. Without a JSON message the text of the body is used, and without that the status text.
func errorMessage(body string, status int) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err == nil {
		if field, found := messageField(parsed); found {
			text := textOf(field)
			details := validationDetails(parsed)
			if len(details) == 0 {
				return text
			}
			return text + ": " + strings.Join(details, "; ")
		}
	}
	if strings.TrimSpace(body) == "" {
		if reason := http.StatusText(status); reason != "" {
			return reason
		}
		return "error"
	}
	runes := []rune(body)
	if len(runes) > maxErrorBodyRunes {
		runes = runes[:maxErrorBodyRunes]
	}
	return string(runes)
}

// messageField returns `message`, or `error` when there is no `message` key at all.
func messageField(parsed map[string]any) (any, bool) {
	if field, found := parsed["message"]; found {
		return field, true
	}
	field, found := parsed["error"]
	return field, found
}

// textOf renders a message as text: strings as they are, any other JSON value as its JSON form.
func textOf(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

// validationDetails lists the entries of the GitHub `errors` array: their `message`, or `field code`.
func validationDetails(parsed map[string]any) []string {
	entries, ok := parsed["errors"].([]any)
	if !ok {
		return nil
	}
	details := make([]string, 0, len(entries))
	for _, entry := range entries {
		switch typed := entry.(type) {
		case string:
			details = append(details, typed)
		case map[string]any:
			if message, ok := typed["message"].(string); ok {
				details = append(details, message)
				continue
			}
			field, hasField := typed["field"].(string)
			code, hasCode := typed["code"].(string)
			if hasField && hasCode {
				details = append(details, field+" "+code)
			}
		}
	}
	return details
}
