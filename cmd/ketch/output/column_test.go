package output

import (
	"reflect"
	"testing"
)

type Item struct {
	Name          string `column:"name"`
	Value         int    `column:"VALUE"`
	UnlabeledData float64
	Omit          string `column:"-"`
	// TODO implement omitempty
}

func TestMarshal(t *testing.T) {
	tests := []struct {
		v           interface{}
		expected    []byte
		err         error
		description string
	}{
		{
			v:           Item{Name: "test", Value: 2, UnlabeledData: 3.1},
			expected:    []byte("name    VALUE    UNLABELED DATA    \ntest    2        3.1"),
			description: "struct",
		},
		{
			v: []Item{{
				Name: "test", Value: 2,
			}, {
				Name: "test-2", Value: 3,
			}},
			expected:    []byte("name      VALUE    UNLABELED DATA    \ntest      2        0                 \ntest-2    3        0"),
			description: "slice",
		},
		{
			v:           &Item{Name: "test", Value: 2},
			expected:    []byte("name    VALUE    UNLABELED DATA    \ntest    2        0"),
			description: "pointer",
		},
	}
	for _, test := range tests {
		c := &Column{}
		res, err := c.Marshal(test.v)
		if err != test.err {
			t.Errorf("expected error %v, got %v", test.err, err)
		}
		if !reflect.DeepEqual(res, test.expected) {
			t.Errorf("test: %s...expected \n%s\n got \n%s\n", test.description, string(test.expected), string(res))
			t.Log(test.expected)
			t.Log(res)
		}
	}
}
