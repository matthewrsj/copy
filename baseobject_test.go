package copy

import "testing"

func TestBaseString(t *testing.T) {
	b := base{path: "foo"}
	if b.String() != "copyObject: foo" {
		t.Errorf("expected 'copyObject: foo' but got '%s'", b.String())
	}
}