package copy

import (
	"io"
	"os"
	"path/filepath"
)

type file struct {
	base
}

func newFile(path string, fi os.FileInfo) file {
	return file{base{path, fi}}
}

// copyTo copies the f.path file to dst location, creating all parent directories
// along the way. This means that directories that did not exist before
// will exist after copying.
func (f file) copyTo(dst string) error {
	// make any parent directories. Assume os.ModePerm
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	// create dst file for write
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer closeFile(df)

	// change dst file to have src mode
	if err = os.Chmod(df.Name(), f.info.Mode()); err != nil {
		return err
	}

	// open source file for read
	sf, err := os.Open(f.path)
	if err != nil {
		return err
	}
	defer closeFile(sf)

	// copy contents
	_, err = io.Copy(df, sf)
	return err
}

func (f file) linkTo(dst string) error {
	if err := os.Link(f.path, dst); err != nil {
		// link failed, might be a directory, fallback to recursive copy
		return f.copyTo(dst)
	}
	return nil
}

func (f file) String() string {
	return "file: " + f.path
}
