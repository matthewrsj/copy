package copy

import "os"

type link struct {
	base
}

func newLink(path string, fi os.FileInfo) link {
	return link{base{path, fi}}
}

// copyTo copies a symlink by replicating the l.path symlink at dst
func (l link) copyTo(dst string) error {
	src, err := os.Readlink(l.path)
	if err != nil {
		return err
	}

	return os.Symlink(src, dst)
}

func (l link) linkOrCopyTo(dst string) error {
	return l.copyTo(dst)
}

func (l link) String() string {
	return "link: " + l.path
}
