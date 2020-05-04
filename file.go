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
func (f file) copyTo(dst string, linkOrCopy bool) error {
	// make any parent directories. Assume os.ModePerm
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	// If the file already exists, check to see if its the same file.  If not, remove it.
	dstInfo, err := os.Stat(dst)
	if err == nil {
		if linkOrCopy && os.SameFile(f.info, dstInfo) {
			return nil
		}
	}

	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	if linkOrCopy {
		err := os.Link(f.path, dst)
		if err == nil {
			return nil
		}
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

func (f file) String() string {
	return "file: " + f.path
}
