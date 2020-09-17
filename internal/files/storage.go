package files

import "io"

// Storage defines the behavior for file operations
// Implementations may be of the time local disk, or cloud storage, etc
type Storage interface {
	Save(path string, file io.Reader) error
	FullPath(path string) string
	Unzip(src, dest, name string) error
	Zip(src, dest, dir, name string) error
	Checkout(src, dest, commit, name string) error
}
