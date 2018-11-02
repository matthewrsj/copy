package copy

import (
	"os"
)

type base struct {
	path string
	info os.FileInfo
}

func (b base) String() string {
	return "copyObject: " + b.path
}

func (b base) Path() string {
	return b.path
}

func (b base) Info() os.FileInfo {
	return b.info
}