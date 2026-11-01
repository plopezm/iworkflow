package open_with_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var launcherAttr = regexp.MustCompile(`data-launcher="([^"]+)"`)

func firstLauncherID(t *testing.T, html string) string {
	t.Helper()
	match := launcherAttr.FindStringSubmatch(html)
	require.NotNil(t, match, html)
	return match[1]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
