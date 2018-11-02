package copy

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mustCreateTestDirectory(t *testing.T, parent, name string) string {
	t.Helper()
	d, err := ioutil.TempDir(parent, name)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func mustCreateTestFile(t *testing.T, path, content string) *os.File {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return f
}

func mustCreateTestLink(t *testing.T, linkname, target string) {
	t.Helper()
	if err := os.Symlink(target, linkname); err != nil {
		t.Fatal(err)
	}
}

func TestNewObject(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "directory")
	f := mustCreateTestFile(t, filepath.Join(d, "file"), "test")
	l := filepath.Join(d, "link")
	mustCreateTestLink(t, l, filepath.Join(d, f.Name()))
	testCases := []struct {
		name, filepath string
		expected       reflect.Type
	}{
		{"directory", d, reflect.TypeOf(directory{})},
		{"file", f.Name(), reflect.TypeOf(file{})},
		{"link", l, reflect.TypeOf(link{})},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := newObject(tc.filepath)
			if err != nil {
				t.Fatal(err)
			}
			actual := reflect.TypeOf(o)
			if actual != tc.expected {
				t.Errorf("expected %s but got %s", tc.expected, actual)
			}
		})
	}
}

func TestNewObjectError(t *testing.T) {
	if _, err := newObject("nofile"); err == nil {
		t.Error("expected error when file did not exist but no error was returned")
	}

	if _, err := newObject("/dev/tty0"); err == nil {
		t.Error("expected error when file was non-regular, directory, or symlink")
	}
}

func mustBeSameFile(t *testing.T, f1, f2 string) {
	t.Helper()
	f1i, err := os.Lstat(f1)
	if err != nil {
		t.Fatal(err)
	}
	f2i, err := os.Lstat(f2)
	if err != nil {
		t.Fatal(err)
	}

	if f1i.Mode() != f2i.Mode() {
		t.Fatalf("%s mode %v does not match %s mode %v", f1, f1i.Mode(), f2, f2i.Mode())
	}

	if f1i.Size() != f2i.Size() {
		t.Fatalf("%s size %v does not match %s size %v", f1, f1i.Size(), f2, f2i.Size())
	}

	if !f1i.Mode().IsRegular() {
		return
	}

	b1, err := ioutil.ReadFile(f1)
	if err != nil {
		t.Fatal(err)
	}

	b2, err := ioutil.ReadFile(f2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b1, b2) {
		t.Fatalf("%s content does not match %s content", f1, f2)
	}
}

func TestCopyAll(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "copyall")
	d2 := mustCreateTestDirectory(t, d, "copyallchild")
	f := mustCreateTestFile(t, filepath.Join(d2, "file1"), "test")
	mustCreateTestLink(t, filepath.Join(d2, "link1"), filepath.Join(d2, "file1"))
	dst := filepath.Join(d, "copyallcopy")

	if err := All(d2, dst); err != nil {
		t.Fatal(err)
	}

	mustBeSameFile(t, d2, dst)
	mustBeSameFile(t, f.Name(), filepath.Join(dst, "file1"))
	mustBeSameFile(t, filepath.Join(d2, "link1"), filepath.Join(dst, "link1"))
}

func TestCopyAllError(t *testing.T) {
	if err := All("none", "none"); err == nil {
		t.Error("expected error when file does not exist but no error was returned")
	}
}

func TestLinkOrCopy(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "copyall")
	f := mustCreateTestFile(t, filepath.Join(d, "file1"), "test")
	dst := filepath.Join(d, "file2")

	if err := LinkOrCopy(f.Name(), dst); err != nil {
		t.Fatal(err)
	}

	mustBeSameFile(t, f.Name(), dst)
}

func TestLinkOrCopyDirectory(t *testing.T) {
	d := mustCreateTestDirectory(t, "", "copyall")
	d2 := mustCreateTestDirectory(t, d, "copyallchild")
	f := mustCreateTestFile(t, filepath.Join(d2, "file1"), "test")
	dst := filepath.Join(d, "copyallcopy")

	if err := LinkOrCopy(d2, dst); err != nil {
		t.Fatal(err)
	}

	mustBeSameFile(t, d2, dst)
	mustBeSameFile(t, f.Name(), filepath.Join(dst, "file1"))
}
