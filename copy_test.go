package copy

import (
	"testing"
	"os"
	"io/ioutil"
	"reflect"
	"path/filepath"
)

func TestNewObject(t *testing.T) {
	d, err := ioutil.TempDir("", "newobject")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(d, "file"), os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	l := filepath.Join(d, "link")
	if err := os.Symlink(filepath.Join(d, f.Name()), l); err != nil {
		t.Fatal(err)
	}
	testCases := []struct{
		name, filepath string
		expected reflect.Type
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

// TODO
// test everything else