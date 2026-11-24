package desktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envOf returns a Getenv function over a fixed map.
func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestUpdateStrategy(t *testing.T) {
	tests := []struct {
		name string
		in   PolicyInput
		want Strategy
	}{
		{name: "windows installer updates itself", in: PolicyInput{GOOS: "windows", Packaged: true}, want: StrategyAuto},
		{
			name: "linux with APPIMAGE set updates itself too",
			in:   PolicyInput{GOOS: "linux", Packaged: true, Getenv: envOf(map[string]string{EnvAppImage: "/opt/iworkflow.AppImage"})},
			want: StrategyAuto,
		},
		{name: "macOS updates itself", in: PolicyInput{GOOS: "darwin", Packaged: true}, want: StrategyAuto},
		{name: "linux archive install updates itself", in: PolicyInput{GOOS: "linux", Packaged: true}, want: StrategyAuto},
		{name: "development run never checks", in: PolicyInput{GOOS: "windows", Packaged: false}, want: StrategyOff},
		{
			name: "IWF_UPDATE_CHECK=off disables windows",
			in:   PolicyInput{GOOS: "windows", Packaged: true, Getenv: envOf(map[string]string{EnvUpdateCheck: "off"})},
			want: StrategyOff,
		},
		{
			name: "IWF_UPDATE_CHECK=OFF disables linux AppImage",
			in: PolicyInput{GOOS: "linux", Packaged: true, Getenv: envOf(map[string]string{
				EnvUpdateCheck: "OFF", EnvAppImage: "/x",
			})},
			want: StrategyOff,
		},
		{
			name: "IWF_UPDATE_CHECK=0 disables darwin",
			in:   PolicyInput{GOOS: "darwin", Packaged: true, Getenv: envOf(map[string]string{EnvUpdateCheck: "0"})},
			want: StrategyOff,
		},
		{
			name: "IWF_UPDATE_CHECK=false disables windows",
			in:   PolicyInput{GOOS: "windows", Packaged: true, Getenv: envOf(map[string]string{EnvUpdateCheck: "false"})},
			want: StrategyOff,
		},
		{
			name: "any other IWF_UPDATE_CHECK value keeps the platform strategy",
			in:   PolicyInput{GOOS: "windows", Packaged: true, Getenv: envOf(map[string]string{EnvUpdateCheck: "on"})},
			want: StrategyAuto,
		},
		{
			name: "APPIMAGE does not change the strategy",
			in:   PolicyInput{GOOS: "linux", Packaged: true, Getenv: envOf(map[string]string{EnvAppImage: ""})},
			want: StrategyAuto,
		},
		{name: "nil Getenv behaves like an empty environment", in: PolicyInput{GOOS: "windows", Packaged: true}, want: StrategyAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UpdateStrategy(tt.in))
		})
	}
}

func TestIWFUpdateCheckDisablesEveryPlatform(t *testing.T) {
	for _, value := range []string{"off", "OFF", "0", "false"} {
		for _, goos := range []string{"windows", "linux", "darwin"} {
			in := PolicyInput{GOOS: goos, Packaged: true, Getenv: envOf(map[string]string{
				EnvUpdateCheck: value, EnvAppImage: "/x",
			})}
			assert.Equal(t, StrategyOff, UpdateStrategy(in), "%s with %s", goos, value)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
	}{
		{name: "minor bump is newer", current: "0.16.0", candidate: "v0.17.0", want: true},
		{name: "numeric not lexical comparison", current: "0.9.0", candidate: "0.10.0", want: true},
		{name: "older major is not newer", current: "1.0.0", candidate: "0.99.99", want: false},
		{name: "same version is not newer", current: "0.16.0", candidate: "0.16.0", want: false},
		{name: "patch bump with leading v is newer", current: "0.16.0", candidate: "v0.16.1", want: true},
		{name: "malformed candidate is never offered", current: "0.16.0", candidate: "nightly", want: false},
		{name: "pre-release candidate is never offered", current: "0.16.0", candidate: "0.17.0-rc.1", want: false},
		{name: "malformed current is never newer", current: "garbage", candidate: "0.17.0", want: false},
		{name: "too many components is malformed", current: "0.16.0", candidate: "0.17.0.1", want: false},
		{name: "overflowing component is malformed", current: "0.16.0", candidate: "99999999999999999999.0.0", want: false},
		{name: "double v is malformed", current: "0.16.0", candidate: "vv0.17.0", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewerVersion(tt.current, tt.candidate))
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		text string
		want Version
		ok   bool
	}{
		{text: "1.2.3", want: Version{1, 2, 3}, ok: true},
		{text: "v10.0.7", want: Version{10, 0, 7}, ok: true},
		{text: " 0.16.0 ", want: Version{0, 16, 0}, ok: true},
		{text: "1.2", ok: false},
		{text: "1.2.x", ok: false},
		{text: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, ok := ParseVersion(tt.text)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLatestReleaseAPIURL(t *testing.T) {
	assert.Equal(t, "plopezm/iworkflow", UpdateRepo)
	assert.Equal(t, "https://api.github.com/repos/plopezm/iworkflow/releases/latest", LatestReleaseAPIURL(UpdateRepo))
}

func TestParseLatestRelease(t *testing.T) {
	tests := []struct {
		name    string
		payload *ReleasePayload
		want    Release
		ok      bool
	}{
		{
			name: "published release is reduced to version and page",
			payload: &ReleasePayload{
				TagName: "v0.17.0", HTMLURL: "https://github.com/x/y/releases/tag/v0.17.0",
			},
			want: Release{Version: "0.17.0", URL: "https://github.com/x/y/releases/tag/v0.17.0"},
			ok:   true,
		},
		{name: "draft is ignored", payload: &ReleasePayload{TagName: "v1.0.0", HTMLURL: "u", Draft: true}},
		{name: "pre-release is ignored", payload: &ReleasePayload{TagName: "v1.0.0", HTMLURL: "u", Prerelease: true}},
		{name: "missing tag is unusable", payload: &ReleasePayload{HTMLURL: "u"}},
		{name: "missing page is unusable", payload: &ReleasePayload{TagName: "v1.0.0"}},
		{name: "nil payload is unusable", payload: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseLatestRelease(tt.payload)
			require.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
