package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iantal/rm/internal/domain"
	"github.com/iantal/rm/internal/files"
	"github.com/iantal/rm/internal/repository"
	"github.com/iantal/rm/internal/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

// Projects is a handler for reading and writing projects to a storage and db
type Projects struct {
	l      *util.StandardLogger
	store  files.Storage
	db     *repository.ProjectDB
	rkHost string
}

// NewProjects creates a handler for projects
func NewProjects(log *util.StandardLogger, store files.Storage, db *repository.ProjectDB, rkHost string) *Projects {
	return &Projects{
		l:      log,
		store:  store,
		db:     db,
		rkHost: rkHost,
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

	if project := p.getProjectForCommit(projectID, commit); project != nil {
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, project.BundlePath)
		return
	}

	if project := p.checkoutCommitForProject(commit, projectID); project != nil {
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, project.BundlePath)
		return
	}

	p.l.WithField("projectID", projectID).Info("Downloading zip from rk")
	projectName, zipPath, err := p.downloadZip(projectID)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": projectID,
			"error":     err,
		}).Error("Could not download zip")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
	}

	zipFile := projectName + ".zip"
	downloadedZip := p.store.FullPath(filepath.Join(zipPath, zipFile))
	err = p.extractZip(downloadedZip, projectID, projectName)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": projectID,
			"zipPath": downloadedZip,
			"error":     err,
		}).Error("Unable to unzip file")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
	}

	if project := p.checkoutCommitForProject(commit, projectID); project != nil {
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, project.BundlePath)
	} else {
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
	}

}

// Gets the project from db for a given commit or nil if not found
func (p *Projects) getProjectForCommit(projectID, commit string) *domain.Project {
	existingProject := p.db.GetProjectByIDAndCommit(projectID, commit)
	if existingProject != nil && existingProject.Name != "" {
		p.l.WithFields(
			logrus.Fields{
				"projectID":   existingProject.ProjectID,
				"projectName": existingProject.Name,
				"commit":      existingProject.CommitHash,
				"zipPath":     existingProject.BundlePath,
			}).Info("Project with commit found")
		return existingProject
	}
	return nil
}

// Checks out the commit for a given project if the project was already downloaded from rk or nil if not found
func (p *Projects) checkoutCommitForProject(commit, projectID string) *domain.Project {
	existingProject := p.db.GetProjectByID(projectID)

	if existingProject == nil || existingProject.UnzippedPath == "" {
		p.l.WithFields(logrus.Fields{
			"projectId": projectID,
			"commit":    commit,
		}).Info("Project is not downloaded or unzipped yet")
		return nil
	}

	commitBundlePath := p.store.FullPath(filepath.Join(projectID, commit))
	err := p.store.Checkout(existingProject.UnzippedPath, commitBundlePath, commit, projectID, existingProject.Name)
	if err != nil {
		p.l.WithFields(
			logrus.Fields{
				"projectID": projectID,
				"commit":    commit,
				"error":     err,
			}).Error("Unable to checkout")
		return nil
	}

	unzippedPath := p.store.FullPath(filepath.Join(projectID, "unzip"))
	commitBundleFile := existingProject.Name + ".bundle"
	bundleFilePath := filepath.Join(commitBundlePath, commitBundleFile)
	project := domain.NewProject(uuid.MustParse(projectID), commit, existingProject.Name, unzippedPath, bundleFilePath)
	p.l.WithFields(
		logrus.Fields{
			"projectID": projectID,
			"commit":    commit,
		}).Debug("Saving to db")
	p.db.AddProject(project)
	return project
}

func (p *Projects) downloadZip(projectID string) (string, string, error) {
	p.l.WithField("projectId", projectID).Info("Getting project name from rk")

	// get projectName from rk
	project, err := p.getProjectName(projectID)
	if err != nil {
		return "", "", xerrors.Errorf("Could not get project name", err)
	}

	projectZipPath := filepath.Join(p.store.FullPath(projectID), "zip")
	if _, err := os.Stat(projectZipPath); os.IsNotExist(err) {
		projectID := project.ProjectID.String()
		resp, err := http.DefaultClient.Get("http://" + p.rkHost + "/api/v1/projects/" + projectID + "/download")
		if err != nil {
			return "", "", err
		}

		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("Expected error code 200 got %d", resp.StatusCode)
		}

		p.saveZip(projectID, project.Name, resp.Body)
		resp.Body.Close()
	}
	return project.Name, projectZipPath, nil
}

func (p *Projects) getProjectName(projectID string) (*domain.Project, error) {
	resp, err := http.DefaultClient.Get("http://" + p.rkHost + "/api/v1/projects/" + projectID)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Expected error code 200 got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	project := &domain.Project{}
	err = json.Unmarshal(body, project)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (p *Projects) saveZip(id, projectName string, r io.ReadCloser) {
	p.l.WithField("projectID", id).Info("Saving project to storage")

	zipFile := projectName + ".zip"
	tempZip := p.store.FullPath(filepath.Join(id, "zip", zipFile))

	err := p.store.Save(tempZip, r)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": id,
			"error":     err,
		}).Error("Unable to save zip")
		return
	}
}

func (p *Projects) extractZip(zipFile, projectID, projectName string) error {
	p.l.WithFields(logrus.Fields{
		"projectID": projectID,
		"path":      filepath.Join(projectID, "unzip"),
	}).Info("Unzipping")

	err := p.store.Unzip(zipFile, p.store.FullPath(filepath.Join(projectID, "unzip")), projectName)
	if err != nil {
		return err
	}

	os.RemoveAll(filepath.Join(projectID, "zip"))
	return nil
}
