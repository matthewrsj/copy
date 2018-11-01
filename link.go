package copy

import "os"

type link struct {
	path string
	info os.FileInfo
}

// copyTo copies a symlink by replicating the l.path symlink at dst
func (l link) copyTo(dst string) error {
	src, err := os.Readlink(l.path)
	if err != nil {
		return err
	}

	return os.Symlink(src, dst)
}

func (l link) String() string {
	return "link: " + l.path
}