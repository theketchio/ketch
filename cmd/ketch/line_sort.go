package main

import (
	"errors"
	"sort"
)

func SortLines(ls [][]string) error {
	if len(ls) < 2 {
		return nil
	}

	for i, j := 0, 1; j < len(ls); {
		if len(ls[i]) != len(ls[j]) {
			return errors.New("lines don't have same number of columns")
		}
		i++
		j++
	}
	rs := &recordSorter{ls}
	sort.Sort(rs)
	return nil
}

// Sorts rows alphabetically by column.
type recordSorter struct {
	rows [][]string
}

func (s *recordSorter) Swap(i, j int) {
	s.rows[j], s.rows[i] = s.rows[i], s.rows[j]
}

func (s *recordSorter) Len() int {
	return len(s.rows)
}

func (s *recordSorter) Less(i, j int) bool {
	for col := 0; col < len(s.rows[i]); col++ {
		if s.rows[i][col] < s.rows[j][col] {
			return true
		}
	}
	return false
}
