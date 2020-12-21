package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_recordSorter(t *testing.T) {
	tt := []struct {
		name     string
		unsorted [][]string
		sorted   [][]string
		wantErr  bool
	}{
		{
			name: "happy path",
			unsorted: [][]string{
				{"x", "z", "y"},
				{"w", "a", "b"},
			},
			sorted: [][]string{
				{"w", "a", "b"},
				{"x", "z", "y"},
			},
		},
		{
			name:     "empty list",
			unsorted: [][]string{},
			sorted:   [][]string{},
		},
		{
			name: "line length mismatch",
			unsorted: [][]string{
				{"x", "z", "y"},
				{"w", "a", "b", "d"},
			},

			wantErr: true,
		},
		{
			name: "sort for subsequent columns",
			unsorted: [][]string{
				{"x", "z", "y", "b"},
				{"x", "z", "y", "c"},
				{"x", "z", "y", "a"},
			},
			sorted: [][]string{
				{"x", "z", "y", "a"},
				{"x", "z", "y", "b"},
				{"x", "z", "y", "c"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := SortLines(tc.unsorted)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.ElementsMatch(t, tc.unsorted, tc.sorted)
		})
	}
}
