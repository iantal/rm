package service

import (
	"github.com/google/uuid"
	"github.com/iantal/rm/internal/domain"
	"github.com/iantal/rm/internal/files"
	"github.com/iantal/rm/internal/repository"
	"github.com/iantal/rm/internal/util"
	"github.com/sirupsen/logrus"
)

type RepositoryManager struct {
	l      *util.StandardLogger
	store  files.Storage
	db     *repository.ProjectDB
	rkHost string
}

func NewRepositoryManager(log *util.StandardLogger, store files.Storage, db *repository.ProjectDB, rkHost string) *RepositoryManager {
	return &RepositoryManager{
		l:      log,
		store:  store,
		db:     db,
		rkHost: rkHost,
	}
}

// Gets the project from db for a given commit and projectId or nil if not found
func (r *RepositoryManager) GetProjectForCommit(projectID, commit string) *domain.Project {
	existingProject := r.db.GetProjectByIDAndCommit(projectID, commit)
	if existingProject != nil && existingProject.BundlePath != "" {
		r.l.WithFields(
			logrus.Fields{
				"projectID":   existingProject.ProjectID,
				"projectName": existingProject.Name,
				"commit":      existingProject.CommitHash,
				"bundlePath":  existingProject.BundlePath,
			}).Info("Project with commit found")
		return existingProject
	}
	return nil
}

func (r *RepositoryManager) SaveToDb(projectName, projectID, commit string) *domain.Project {
	up := r.store.UnzipPath(projectID)
	bp := r.store.BundleFilePath(projectID, commit, projectName)
	project := domain.NewProject(uuid.MustParse(projectID), commit, projectName, up, bp)
	r.db.AddProject(project)
	return project
}
