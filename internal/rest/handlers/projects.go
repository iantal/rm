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

	p.l.WithField("projectID", projectID).Info("Downloading")

	// return the bundle for commit if exists
	existingProject := p.db.GetProjectByIDAndCommit(projectID, commit)
	if existingProject != nil && existingProject.Name != "" {
		p.l.WithFields(
			logrus.Fields{
				"projectID":   existingProject.ProjectID,
				"projectName": existingProject.Name,
				"commit":      existingProject.CommitHash,
				"zipPath":     existingProject.ZippedPath,
			}).Info("Project with commit found")
		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+existingProject.Name+".bundle\"")
		http.ServeFile(rw, r, existingProject.ZippedPath)
		return
	}

	// checkout new commit if project is downloaded
	existingProject = p.db.GetProjectByID(projectID)
	if existingProject != nil && existingProject.UnzipedPath != "" {
		p.l.WithField("projectID", projectID).Info("Project found")
		commitBundle := existingProject.Name + ".bundle"
		commitBundlePath := p.store.FullPath(filepath.Join(projectID, commit))

		err := p.store.Checkout(existingProject.UnzipedPath, commitBundlePath, commit, existingProject.ProjectID.String(), existingProject.Name)
		if err != nil {
			p.l.WithFields(
				logrus.Fields{
					"projectID": projectID,
					"commit":    commit,
					"error":     err,
				}).Error("Unable to checkout")
			return
		}

		project := domain.NewProject(uuid.MustParse(projectID), commit, existingProject.Name, existingProject.UnzipedPath, filepath.Join(commitBundlePath, commitBundle))
		p.l.WithFields(
			logrus.Fields{
				"projectID": projectID,
				"commit":    commit,
			}).Debug("Saving to db")
		p.db.AddProject(project)

		rw.Header().Set("Content-type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
		http.ServeFile(rw, r, existingProject.ZippedPath)
		return
	}

	p.l.WithFields(logrus.Fields{
		"projectID": projectID,
		"commit":    commit,
	}).Info("Getting project name from rk")
	// get projectName from rk
	project, err := p.getProjectName(projectID)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": projectID,
			"commit":    commit,
			"error":     err,
		}).Error("Could not get project name")
		rw.WriteHeader(http.StatusInternalServerError)
		util.ToJSON(&GenericError{Message: "Project not found"}, rw)
		return
	}

	p.l.Info("Downloading project")
	// download the project from rk
	projectPath := filepath.Join(p.store.FullPath(projectID), "zip")
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		err = p.downloadRepository(project, commit)
		if err != nil {
			p.l.WithFields(logrus.Fields{
				"projectID": projectID,
				"commit":    commit,
				"error":     err,
			}).Error("Could not download zip from rk for project")
			rw.WriteHeader(http.StatusInternalServerError)
			util.ToJSON(&GenericError{Message: "Could not download project"}, rw)
		}

	}

	unzippedPath := p.store.FullPath(filepath.Join(projectID, "unzip"))
	commitBundle := existingProject.Name + ".bundle"
	commitBundlePath := p.store.FullPath(filepath.Join(projectID, commit))

	err = p.store.Checkout(unzippedPath, commitBundlePath, commit, projectID, project.Name)
	if err != nil {
		p.l.WithFields(
			logrus.Fields{
				"projectID": projectID,
				"commit":    commit,
				"error":     err,
			}).Error("Unable to checkout")
		return
	}

	project = domain.NewProject(uuid.MustParse(projectID), commit, project.Name, unzippedPath, filepath.Join(commitBundlePath, commitBundle))
	p.l.WithFields(
		logrus.Fields{
			"projectID": projectID,
			"commit":    commit,
		}).Info("Saving to db")
	p.db.AddProject(project)

	rw.Header().Set("Content-type", "application/octet-stream")
	rw.Header().Set("Content-Disposition", "attachment; filename=\""+project.Name+".bundle\"")
	http.ServeFile(rw, r, project.ZippedPath)

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

func (p *Projects) downloadRepository(project *domain.Project, commit string) error {
	projectID := project.ProjectID.String()
	resp, err := http.DefaultClient.Get("http://" + p.rkHost + "/api/v1/projects/" + projectID + "/download")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Expected error code 200 got %d", resp.StatusCode)
	}

	p.save(projectID, project.Name, resp.Body, commit)
	resp.Body.Close()

	return nil
}

func (p *Projects) save(id, projectName string, r io.ReadCloser, commit string) {
	p.l.WithFields(logrus.Fields{
		"projectID": id,
		"commit":    commit,
	}).Info("Saving project to storage")
	unzippedPath := p.store.FullPath(filepath.Join(id, "unzip"))

	zipFile := projectName + ".zip"
	tempZip := p.store.FullPath(filepath.Join(id, "zip", zipFile))

	err := p.store.Save(tempZip, r)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": id,
			"commit":    commit,
			"error":     err,
		}).Error("Unable to save file")
		return
	}

	p.l.WithFields(logrus.Fields{
		"projectID": id,
		"commit":    commit,
		"path":      filepath.Join(id, "unzip"),
	}).Info("Unzipping")
	err = p.store.Unzip(tempZip, p.store.FullPath(filepath.Join(id, "unzip")), projectName)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": id,
			"commit":    commit,
			"error":     err,
		}).Error("Unable to unzip file")
		return
	}

	os.RemoveAll(filepath.Join(id, "zip"))

	commitZip := projectName + ".bundle"
	commitZipPath := p.store.FullPath(filepath.Join(id, commit))

	err = p.store.Checkout(unzippedPath, commitZipPath, commit, id, projectName)
	if err != nil {
		p.l.WithFields(logrus.Fields{
			"projectID": id,
			"commit":    commit,
			"error":     err,
		}).Error("Unable to checkout")
		return
	}

	// project := domain.NewProject(uuid.MustParse(id), commit, projectName, unzippedPath, filepath.Join(commitZipPath, commitZip))
	// p.l.WithFields(logrus.Fields{
	// 	"projectID":   id,
	// 	"commit":      commit,
	// 	"projectName": projectName,
	// }).Info("Saving project to db")
	// p.db.AddProject(project)
}
