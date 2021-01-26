package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogLineParser(t *testing.T) {
	tt := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "status",
			input:    `{"status":"Pushing","progressDetail":{"current":512,"total":11878},"progress":"[==\u003e                                                ]     512B/11.88kB","id":"6570a617e60b"}`,
			expected: "Pushing, current 512, total 11878, progress: [==>                                                ]     512B/11.88kB, id: 6570a617e60b\n",
		},
		{
			name:     "stream",
			input:    `{"stream":" ---\u003e Running in bd81c51c661e\n"}`,
			expected: " ---> Running in bd81c51c661e\n",
		},
		{
			name:     "aux with sha",
			input:    `{"aux":{"ID":"sha256:8741ab561337f6795e5ae0206279dae98bc4292c746e7728ffb9967a5d5eb1e8"}}`,
			expected: "ID: sha256:8741ab561337f6795e5ae0206279dae98bc4292c746e7728ffb9967a5d5eb1e8\n",
		},
		{
			name:     "aux end push",
			input:    `{"progressDetail":{},"aux":{"Tag":"v0.1","Digest":"sha256:85be26d33c69f346e9b9ba4959f4d7bc7c801f7ebe5ea9c8b274a22860a91d5b","Size":3042}}`,
			expected: "Tag: v0.1, Digest: sha256:85be26d33c69f346e9b9ba4959f4d7bc7c801f7ebe5ea9c8b274a22860a91d5b, Size: 3042\n",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rl, err := NewLine([]byte(tc.input))
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.expected, rl.String())
		})
	}
}
