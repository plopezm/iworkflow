package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/application"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
)

type fakeNativePicker struct {
	path string
	err  error
}

func (f fakeNativePicker) Pick(context.Context) (string, error) { return f.path, f.err }

func TestPickExecutable(t *testing.T) {
	boom := errors.New("boom")
	tests := map[string]struct {
		picker  fakeNativePicker
		want    string
		wantErr error
	}{
		"returns the chosen path":       {picker: fakeNativePicker{path: " /usr/bin/code\n"}, want: "/usr/bin/code"},
		"cancel is an empty path":       {picker: fakeNativePicker{}, want: ""},
		"unavailable is kept as is":     {picker: fakeNativePicker{err: openerrors.ErrNativePickerUnavailable}, wantErr: openerrors.ErrNativePickerUnavailable},
		"other failures are propagated": {picker: fakeNativePicker{err: boom}, wantErr: boom},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := application.NewPickExecutable(tc.picker).Execute(context.Background())
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
