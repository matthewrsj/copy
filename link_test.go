package copy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkString(t *testing.T) {
	l := link{base{path: "foo"}}
	ls := l.String()
	if ls != "link: foo" {
		t.Errorf("expected 'link: foo' but got '%s'", ls)
	}
}

func TestLinkPath(t *testing.T) {
	l := link{base{path: "foo"}}
	lp := l.Path()
	if lp != "foo" {
		t.Errorf("expected 'foo' but got '%s'", lp)
	}
}

func TestLinkInfo(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directory")
	f := mustCreateTestFile(t, filepath.Join(d, "file"), "test")
	l := filepath.Join(d, "link")
	mustCreateTestLink(t, l, filepath.Join(d, f.Name()))

	le, err := os.Lstat(l)
	if err != nil {
		t.Fatal(err)
	}
	lo, err := newObject(l)
	if err != nil {
		t.Fatal(err)
	}
	li := lo.Info()
	// smoke test
	if li.Name() != le.Name() {
		t.Errorf("expected %s but got %s", le.Name(), li.Name())
	}
}

func TestLinkCopyToError(t *testing.T) {
	l := link{base{path: "foo"}}
	if err := l.copyTo("nowhere", false); err == nil {
		t.Error("expected error when file did not exist but no error was returned")
	}
}
