package copy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileString(t *testing.T) {
	f := file{base{path: "foo"}}
	if fs := f.String(); fs != "file: foo" {
		t.Errorf("expected 'file: foo' but got '%s'", fs)
	}
}

func TestFilePath(t *testing.T) {
	f := file{base{path: "foo"}}

	if fp := f.Path(); fp != "foo" {
		t.Errorf("expected 'foo' but got '%s'", fp)
	}
}

func TestFileInfo(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directory")
	ft := mustCreateTestFile(t, filepath.Join(d, "file"))

	fo, err := newObject(ft.Name())
	if err != nil {
		t.Fatal(err)
	}

	fe, err := os.Lstat(ft.Name())
	if err != nil {
		t.Fatal(err)
	}

	// smoke test
	if fi := fo.Info(); fi.Name() != fe.Name() {
		t.Errorf("expected %s but got %s", fe.Name(), fi.Name())
	}
}

func TestFileCopyToError(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directoryinfo")
	fe := mustCreateTestFile(t, filepath.Join(d, "file"))

	fo, err := newObject(fe.Name())
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(fe.Name()); err != nil {
		t.Fatal(err)
	}

	if err := fo.copyTo("nowhere", false); err == nil {
		t.Error("expected error when file did not exist but no error was returned")
	}
}
