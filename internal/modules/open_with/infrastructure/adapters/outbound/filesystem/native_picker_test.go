package filesystem

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
)

func TestPickerCommandsPerSystem(t *testing.T) {
	require.Equal(t, "powershell", pickerCommands("windows")[0].name)
	require.Equal(t, "osascript", pickerCommands("darwin")[0].name)
	linux := pickerCommands("linux")
	require.Equal(t, "zenity", linux[0].name)
	require.Equal(t, "kdialog", linux[1].name)
}

func TestNativePickerUnavailableWithoutProgram(t *testing.T) {
	picker := &NativePicker{
		commands: pickerCommands("linux"),
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
	_, err := picker.Pick(context.Background())
	require.ErrorIs(t, err, openerrors.ErrNativePickerUnavailable)
}

func TestNativePickerRunsProgram(t *testing.T) {
	tests := map[string]struct {
		args    []string
		cancel  []int
		want    string
		wantErr error
	}{
		"prints the path": {args: []string{"-c", "printf '/usr/bin/code\\n'"}, want: "/usr/bin/code"},
		"cancel code":     {args: []string{"-c", "exit 1"}, cancel: []int{1}, want: ""},
		"other failure":   {args: []string{"-c", "exit 3"}, cancel: []int{1}, wantErr: openerrors.ErrNativePickerUnavailable},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			picker := &NativePicker{
				commands: []pickerCommand{{name: "sh", args: tc.args, cancelCodes: tc.cancel}},
				lookPath: func(n string) (string, error) { return "/bin/" + n, nil },
			}
			got, err := picker.Pick(context.Background())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNativePickerUnavailableWithoutDisplay(t *testing.T) {
	ran := false
	picker := &NativePicker{
		commands: pickerCommands("linux"),
		lookPath: func(n string) (string, error) { ran = true; return "/usr/bin/" + n, nil },
		display:  func() bool { return false },
	}
	_, err := picker.Pick(context.Background())
	require.ErrorIs(t, err, openerrors.ErrNativePickerUnavailable)
	require.False(t, ran, "a dialog that needs a display must not even be looked up without one")
}

func TestNativePickerUsesDisplayDialogWhenThereIsOne(t *testing.T) {
	picker := &NativePicker{
		commands: []pickerCommand{{name: "sh", args: []string{"-c", "printf /x"}, needsDisplay: true}},
		lookPath: func(n string) (string, error) { return "/bin/" + n, nil },
		display:  func() bool { return true },
	}
	got, err := picker.Pick(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/x", got)
}

func TestHasDisplay(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		want bool
	}{
		"none":    {env: map[string]string{}, want: false},
		"x11":     {env: map[string]string{"DISPLAY": ":0"}, want: true},
		"wayland": {env: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, want: true},
		"blank":   {env: map[string]string{"DISPLAY": " "}, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, hasDisplay(func(k string) string { return tc.env[k] }))
		})
	}
}
