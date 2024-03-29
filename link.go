package copy

import (
	"os"

	"github.com/pkg/errors"
)

type link struct {
	base
}

func newLink(path string, fi os.FileInfo) link {
	return link{base{path, fi}}
}

// copyTo copies a symlink by replicating the l.path symlink at dst
func (l link) copyTo(dst string, linkOrCopy bool) error {
	src, err := os.Readlink(l.path)
	if err != nil {
		return errors.Wrapf(err, "ReadLink(%s)", l.path)
	}

	// If the link already exists, check to see if it's the same link. If not, remove it.
	dstInfo, err := os.Stat(dst)
	if err == nil && dstInfo.Mode()&os.ModeSymlink != 0 {
		dstLink, err := os.Readlink(dst)
		if err == nil && dstLink == src {
			return nil
		}
	}

	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "Remove(%s)", dst)
	}

	return errors.Wrapf(os.Symlink(src, dst), "Symlink(%s,%s)", src, dst)
}

func (l link) String() string {
	return "link: " + l.path
}
