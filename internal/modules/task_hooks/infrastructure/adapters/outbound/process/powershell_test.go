package process

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotePowerShellDoublesEverySingleQuote(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"plain":         {"abc", "'abc'"},
		"spaces":        {"a b; $HOME", "'a b; $HOME'"},
		"ascii quote":   {"it's", "'it''s'"},
		"smart left":    {"a‘b", "'a‘‘b'"},
		"smart right":   {"a’b", "'a’’b'"},
		"low and high":  {"‚‛", "'‚‚‛‛'"},
		"double quotes": {`say "hi"`, `'say "hi"'`},
		"empty":         {"", "''"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, quotePowerShell(tc.in))
		})
	}
}

func TestPowerShellScriptCallsTheLineWithTheQuotedArguments(t *testing.T) {
	got := powershellScript("codegraph init ; codegraph index", []string{"one", "it's"})
	want := "& {\ncodegraph init ; codegraph index\n} 'one' 'it''s'\n$ok = $?\nif ($ok) { exit 0 }\nif ($LASTEXITCODE) { exit $LASTEXITCODE }\nexit 1\n"
	assert.Equal(t, want, got)
}

func TestPowerShellScriptWithoutArgumentsHasNoLiterals(t *testing.T) {
	assert.Contains(t, powershellScript("echo hi", nil), "echo hi\n}\n$ok = $?")
}

// A line that reads its arguments, by $args or by splatting them, must not get @args added again; one that does not
// read them must. The check runs against the PowerShell markers on every platform.
func TestPowerShellPositionalsTellAReadingLineFromANonReadingOne(t *testing.T) {
	cases := map[string]struct {
		command string
		reads   bool
	}{
		"splat":        {"echo @args", true},
		"index":        {"echo $args[0]", true},
		"whole array":  {"Write-Output $args", true},
		"pipeline in":  {"$input | Sort-Object", true},
		"no arguments": {"codegraph init ; codegraph index", false},
		"bash style":   {`echo "$@"`, false},
		"other var":    {"echo $env:HOME", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.reads, readsAny(tc.command, powershellPositionals))
		})
	}
}
