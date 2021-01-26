package errors

import "testing"

type errTestType string

func (et errTestType) Error() string { return string(et) }

const errTest errTestType = "some error"

func TestWrapError(t *testing.T) {

	err := Wrap(errTest, "whoops %s", "hello")
	if err == nil {
		t.Fatal("shouldn't be nil")
	}
	expected := `message: "whoops hello"; error: "some error"; file: errors_test.go; line: 13`
	actual := err.Error()
	if actual != expected {
		t.Logf("expected: %q", expected)
		t.Logf("actual  : %q", actual)
		t.Fatal("didn't get expected error")
	}

}
