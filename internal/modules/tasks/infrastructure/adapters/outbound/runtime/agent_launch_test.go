package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/env"
	"iworkflow/internal/modules/tasks/domain/models"
)

func uint32Ptr(value uint32) *uint32 {
	return &value
}

// Rust: agent_process.rs::scoped_launch_wraps_the_shell_in_systemd_run_when_limited
func TestScopedLaunchWrapsTheShellInSystemdRunWhenLimited(t *testing.T) {
	limits := models.ResourceLimits{MemoryMB: uint32Ptr(1024), CPUPercent: uint32Ptr(200)}

	program, args := scopedLaunch(limits, true, "/bin/sh", []string{"-l", "-c", "exec 'agent'"})

	assert.Equal(t, "systemd-run", program)
	assert.Equal(t, []string{
		"--user", "--scope", "--quiet",
		"-p", "MemoryMax=1024M",
		"-p", "CPUQuota=200%",
		"--", "/bin/sh", "-l", "-c", "exec 'agent'",
	}, args)
}

// Rust: agent_process.rs::scoped_launch_leaves_the_shell_untouched_without_limits_or_systemd
func TestScopedLaunchLeavesTheShellUntouchedWithoutLimitsOrSystemd(t *testing.T) {
	cases := []struct {
		name    string
		limits  models.ResourceLimits
		systemd bool
	}{
		{name: "limits without systemd", limits: models.ResourceLimits{MemoryMB: uint32Ptr(1024)}, systemd: false},
		{name: "systemd without limits", limits: models.ResourceLimits{}, systemd: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, args := scopedLaunch(tc.limits, tc.systemd, "/bin/sh", []string{"-l"})

			assert.Equal(t, "/bin/sh", program)
			assert.Equal(t, []string{"-l"}, args)
		})
	}
}

// Rule from source: extra variables of the launch spec are applied after the session environment,
// so they override it per key and no key appears twice.
func TestMergeEnvLetsExtrasOverrideTheSessionValues(t *testing.T) {
	base := []env.Var{{Key: "HOME", Value: "/home/u"}, {Key: "PATH", Value: "/usr/bin"}}
	extra := []env.Var{{Key: "PATH", Value: "/opt/agent/bin"}, {Key: "AGENT_FLAG", Value: "1"}}

	merged := mergeEnv(base, extra)

	assert.Equal(t, []env.Var{
		{Key: "HOME", Value: "/home/u"},
		{Key: "PATH", Value: "/opt/agent/bin"},
		{Key: "AGENT_FLAG", Value: "1"},
	}, merged)
}

// Rule from source: the parent environment is read as KEY=VALUE, split on the first '='. Entries
// without a usable key (Windows "=C:=" drive variables, entries with no '=') are skipped.
func TestParseEnvironSplitsOnTheFirstEqualsAndSkipsKeylessEntries(t *testing.T) {
	parsed := parseEnviron([]string{"A=1", "B=x=y", "=C:=C:\\", "NOEQUALS", ""})

	assert.Equal(t, []env.Var{{Key: "A", Value: "1"}, {Key: "B", Value: "x=y"}}, parsed)
}

// Rule from source: agent_process.rs read_events uses String::from_utf8_lossy, so invalid bytes become
// U+FFFD and valid text around them is kept. Each invalid byte becomes one replacement here; Rust
// replaces a truncated multi-byte sequence with a single one (see report, behavior differences).
func TestLossyUTF8ReplacesInvalidBytesAndKeepsValidText(t *testing.T) {
	assert.Equal(t, "a\uFFFDb", lossyUTF8([]byte("a\xffb")))
	assert.Equal(t, "ñandú", lossyUTF8([]byte("ñandú")))
	assert.Equal(t, "\uFFFD\uFFFD", lossyUTF8([]byte("\xff\xfe")))
}
