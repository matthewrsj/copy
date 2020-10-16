package copy

import (
	"os"
	"testing"
)

const _testName = "foo"

func TestDirectoryString(t *testing.T) {
	d := directory{base{path: _testName}}
	if ds := d.String(); ds != "directory: "+_testName {
		t.Errorf("expected 'directory: foo' but got '%s'", ds)
	}
}

func TestDirectoryPath(t *testing.T) {
	d := directory{base{path: _testName}}
	if dp := d.Path(); dp != _testName {
		t.Errorf("expected 'foo' but got '%s'", dp)
	}
}

func TestDirectoryInfo(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directoryinfo")

	de, err := os.Lstat(d)
	if err != nil {
		t.Fatal(err)
	}

	do, err := newObject(d)
	if err != nil {
		t.Fatal(err)
	}

	// smoke test
	if di := do.Info(); di.Name() != de.Name() {
		t.Errorf("expected %s but got %s", de.Name(), di.Name())
	}
}

func TestDirectoryCopyToError(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directoryinfo")

	do, err := newObject(d)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(d); err != nil {
		t.Fatal(err)
	}

	if err := do.copyTo("nowhere", false); err == nil {
		t.Error("expected error when file did not exist but no error was returned")
	}
}
