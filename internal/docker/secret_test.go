package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_makeK8sName(t *testing.T) {
	tt := []struct {
		name     string
		in       []string
		expected string
		wantErr  bool
	}{
		{
			name: "happy path",
			in: []string{
				"oNe.",
				"zip",
			},
			expected: "one2ezip",
		},
		{
			name: "invalid char",
			in: []string{
				"weird©Name",
				"foöo",
			},
			wantErr: true,
		},
		{
			name: "too long",
			in: []string{
				func() string {
					var b strings.Builder
					for i := 0; i <= maxNameLength; i++ {
						b.WriteByte('x')
					}
					return b.String()
				}(),
				"x.x",
			},
			wantErr: true,
		},
		{
			name:    "empty input",
			wantErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ps, err := makeK8sName(tc.in...)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, *ps, tc.expected)
		})
	}

}
