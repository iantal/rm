package domain

import (
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

// Project defines data related to a project repository
type Project struct {
	gorm.Model  `json:"-"`
	ProjectID   uuid.UUID `gorm:"type:uuid;primary_key;" json:"projectId"`
	CommitHash  string    `gorm:"primary_key" json:"commit,omitempty"`
	Name        string    `json:"name,omitempty"`
	UnzipedPath string    `json:"unzip,omitempty"`
	ZippedPath  string    `json:"zip,omitempty"`
}

// NewProject creates an instance of Project
func NewProject(id uuid.UUID, commit, name, unzipped, zipped string) *Project {
	return &Project{
		ProjectID:   id,
		CommitHash:  commit,
		Name:        name,
		UnzipedPath: unzipped,
		ZippedPath:  zipped,
	}
}
