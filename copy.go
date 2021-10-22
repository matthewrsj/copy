// Package copy provides functionality to recursively copy files, directories or links. When copying it attempts a
// hardline before falling back to the more expensive direct copy if the link fails.
package copy

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

// interface for copying files, directories, or links
type copyObject interface {
	copyTo(dst string, linkOrCopy bool) error
	Path() string
	Info() os.FileInfo
}

// create a new object based on what type of file it is
func newObject(path string) (copyObject, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Lstat(%s)", path)
	}

	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return newLink(path, fi), nil
	case fi.IsDir():
		return newDirectory(path, fi), nil
	case fi.Mode().IsRegular():
		return newFile(path, fi), nil
	default:
		return nil, fmt.Errorf("unsupported file type %s", fi.Mode().String())
	}
}

// All copies the src file to the dst path.
func All(src, dst string) error {
	obj, err := newObject(src)
	if err != nil {
		return errors.Wrapf(err, "newObject(%s)", src)
	}

	if err = obj.copyTo(dst, false); err != nil {
		return errors.Wrapf(err, "copyTo(%s,%t)", dst, false)
	}

	return nil
}

// LinkOrCopy first attempts to hardlink src to dst and falls back
// to a regular recursive copy if that fails. This is useful when
// you might be copying over partition boundaries where a link will
// fail.
func LinkOrCopy(src, dst string) error {
	obj, err := newObject(src)
	if err != nil {
		return errors.Wrapf(err, "newObject(%s)", src)
	}

	if err = obj.copyTo(dst, true); err != nil {
		return errors.Wrapf(err, "copyTo(%s,%t)", dst, true)
	}

	return nil
}

// internal function to throw away file close errors in deferred
// functions
func closeFile(f *os.File) {
	_ = f.Close()
}
