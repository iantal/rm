package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iantal/rm/internal/files"
	"github.com/iantal/rm/internal/repository"
	"github.com/iantal/rm/internal/service"
	"github.com/iantal/rm/internal/util"
	"github.com/sirupsen/logrus"
)

// Projects is a handler for reading and writing projects to a storage and db
type Projects struct {
	l                 *util.StandardLogger
	repositoryManager *service.RepositoryManager
}

// NewProjects creates a handler for projects
func NewProjects(log *util.StandardLogger, store files.Storage, db *repository.ProjectDB, rkHost string) *Projects {
	rm := service.NewRepositoryManager(log, store, db, rkHost)

	return &Projects{
		l:                 log,
		repositoryManager: rm,
	}
}

// GenericError represents an error of the system
type GenericError struct {
	Message string `json:"message"`
}

func (p *Projects) Download(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]
	commit := vars["commit"]

	// 0. project with commit already exists
	if project := p.repositoryManager.GetProjectForCommit(projectID, commit); project != nil {
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, project.BundlePath)
		return
	}

	// get projectName from rk
	projectName, err := p.repositoryManager.GetProjectName(projectID)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": projectID,
			"error":     err,
		}).Error("Could not get project name")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
		return
	}

	// 1. is downloaded, just checkout the commit
	if p.repositoryManager.IsDownloaded(projectID, projectName) {
		err = p.repositoryManager.CheckoutCommit(commit, projectID, projectName)
		if err != nil {
			p.l.WithFields(
				logrus.Fields{
					"projectID":   projectID,
					"commit":      commit,
					"projectName": projectName,
					"error":       err,
				}).Error("Unable to checkout")
			rw.WriteHeader(http.StatusInternalServerError)
			util.ToJSON(&GenericError{Message: "Project not found"}, rw)
			return
		}
		project := p.repositoryManager.SaveToDb(projectName, projectID, commit)
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, project.BundlePath)
		return
	}

	// 2. not downloaded => download from rk
	err = p.repositoryManager.DownloadZip(projectID, projectName)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": projectID,
			"error":     err,
		}).Error("Could not download project from rk")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
		return
	}

	// 3. checkout commit
	err = p.repositoryManager.CheckoutCommit(commit, projectID, projectName)
	if err != nil {
		p.l.WithFields(
			logrus.Fields{
				"projectID":   projectID,
				"commit":      commit,
				"projectName": projectName,
				"error":       err,
			}).Error("Unable to checkout")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
		return
	}
	project := p.repositoryManager.SaveToDb(projectName, projectID, commit)
	rw.Header().Set("Content-type", "application/octet-stream")
	rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
	http.ServeFile(rw, r, project.BundlePath)
}
