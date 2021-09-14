package copy

import "os"

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
		return err
	}
	// If the link already exists, check to see if its the same link.  If not, remove it.
	dstInfo, err := os.Stat(dst)
	if err == nil && dstInfo.Mode()&os.ModeSymlink != 0 {
		dstLink, err := os.Readlink(dst)
		if err == nil && dstLink == src {
			return nil
		}
	}

	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Symlink(src, dst)
}

func (l link) String() string {
	return "link: " + l.path
}
