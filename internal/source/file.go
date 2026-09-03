// Package source fetches the raw cluster-autoscaler status document, either
// from the Kubernetes API or from a local file.
package source

import (
	"context"
	"os"
)

// File reads the status document from disk, which makes it possible to run the
// exporter against a captured document with no cluster access.
type File struct {
	path string
}

// NewFile returns a source reading the status document at path.
func NewFile(path string) *File {
	return &File{path: path}
}

// Fetch implements the collector's Source interface.
func (f *File) Fetch(_ context.Context) ([]byte, error) {
	return os.ReadFile(f.path)
}
