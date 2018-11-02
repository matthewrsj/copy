package copy

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

type directory struct {
	base
}

func newDirectory(path string, fi os.FileInfo) directory {
	return directory{base{path, fi}}
}

// copyTo recursively copies directories from d.path to dst
func (d directory) copyTo(dst string) error {
	// create new directory with source mode
	if err := os.MkdirAll(dst, d.info.Mode()); err != nil {
		return err
	}

	// get all children
	children, err := ioutil.ReadDir(d.path)
	if err != nil {
		return err
	}

	// copy each child recursively
	for _, child := range children {
		childSrc := filepath.Join(d.path, child.Name())
		childDst := filepath.Join(dst, child.Name())
		obj, err := newObject(childSrc)
		if err != nil {
			return err
		}
		if err = obj.copyTo(childDst); err != nil {
			return err
		}
	}

	// successful
	return nil
}

func (d directory) String() string {
	return "directory: " + d.path
}
