package domain

import (
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

// DownloadStatus defines data related to the download status of a project
type DownloadStatus struct {
	gorm.Model   `json:"-"`
	ProjectID    uuid.UUID `gorm:"type:uuid;primary_key;" json:"projectId"`
	UnzippedPath string    `json:"unzip,omitempty"`
}

// NewDownloadStatus creates an instance of DownloadStatus
func NewDownloadStatus(id uuid.UUID, commit, name, unzipped, zipped string) *Project {
	return &Project{
		ProjectID:    id,
		CommitHash:   commit,
		Name:         name,
		UnzippedPath: unzipped,
		BundlePath:   zipped,
	}
}
