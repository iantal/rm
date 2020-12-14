package files

import "io"

// Storage defines the behavior for file operations
// Implementations may be of the time local disk, or cloud storage, etc
type Storage interface {
	FullPath(path string) string
	ProjectPath(projectID string) string
	CommitPath(projectID, commit string) string
	BundleFilePath(projectID, commit, projectName string) string
	ZipFilePath(projectID, projectName string) string
	UnzipPath(projectID string) string

	Save(path string, file io.Reader) error
	Unzip(src, dest, name string) error
	Checkout(src, dest, commit, projectID, name string) error
}
