package copy

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
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
		return errors.Wrapf(err, "MkdirAll(%s,%s)", filepath.Dir(dst), os.ModePerm.String())
	}

	// If the file already exists, check to see if its the same file.  If not, remove it.
	dstInfo, err := os.Stat(dst)
	if err == nil {
		if linkOrCopy && os.SameFile(f.info, dstInfo) {
			return nil
		}
	}

	if err = os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "Remove(%s)", dst)
	}

	if linkOrCopy {
		// linkOrCopy is set, which means attempt a link first
		if err = os.Link(f.path, dst); err == nil {
			// successfully linked, return from function
			return nil
		} // link failed, continue to copy
	}

	// create dst file for write
	df, err := os.Create(dst)
	if err != nil {
		return errors.Wrapf(err, "Create(%s)", dst)
	}

	defer closeFile(df)

	// change dst file to have src mode
	if err = os.Chmod(df.Name(), f.info.Mode()); err != nil {
		return errors.Wrapf(err, "Chmod(%s,%s)", df.Name(), f.info.Mode().String())
	}

	// open source file for read
	sf, err := os.Open(f.path)
	if err != nil {
		return errors.Wrapf(err, "Open(%s)", f.path)
	}

	defer closeFile(sf)

	// copy contents
	_, err = io.Copy(df, sf)

	return errors.Wrapf(err, "Copy(%s,%s)", df.Name(), sf.Name())
}

func (f file) String() string {
	return "file: " + f.path
}
