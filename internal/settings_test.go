package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeSettings_Durations(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    time.Duration
		wantErr string
	}{
		{name: "duration", value: 30 * time.Second, want: 30 * time.Second},
		{name: "integer zero", value: 0, want: 0},
		{name: "float zero", value: 0.0, want: 0},
		{name: "integer without units", value: 30, wantErr: "expected a duration with units, such as 30s, got 30"},
		{name: "float without units", value: 0.5, wantErr: "expected a duration with units, such as 30s, got 0.5"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				dst := struct {
					Interval time.Duration `config:"interval"`
				}{Interval: time.Hour}

				err := DecodeSettings(&map[string]any{"interval": tt.value}, &dst)

				if tt.wantErr != "" {
					assert.ErrorContains(t, err, tt.wantErr)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tt.want, dst.Interval)
			},
		)
	}
}
