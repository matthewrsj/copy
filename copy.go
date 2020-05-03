package copy

import (
	"errors"
	"os"
)

// interface for copying files, directories, or links
type copyObject interface {
	copyTo(dst string) error
	linkOrCopyTo(dst string) error
	Path() string
	Info() os.FileInfo
}

// create a new object based on what type of file it is
func newObject(path string) (copyObject, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return newLink(path, fi), nil
	case fi.IsDir():
		return newDirectory(path, fi), nil
	case fi.Mode().IsRegular():
		return newFile(path, fi), nil
	default:
		return nil, errors.New("unsupported file type")
	}
}

// All copies the src file to the dst path.
func All(src, dst string) error {
	obj, err := newObject(src)
	if err != nil {
		return err
	}
	return obj.copyTo(dst)
}

// LinkOrCopy first attempts to hardlink src to dst and falls back
// to a regular recursive copy if that fails. This is useful when
// you might be copying over partition boundaries where a link will
// fail.
func LinkOrCopy(src, dst string) error {
	obj, err := newObject(src)
	if err != nil {
		return err
	}
	return obj.linkOrCopyTo(dst)
}

// internal function to throw away file close errors in deferred
// functions
func closeFile(f *os.File) {
	_ = f.Close()
}
