package service

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/iantal/rm/internal/domain"
	"github.com/sirupsen/logrus"
)

func (r *RepositoryManager) IsDownloaded(projectID, projectName string) bool {
	zipPath := r.store.ZipFilePath(projectID, projectName)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func (r *RepositoryManager) DownloadZip(projectID, projectName string) error {
	r.l.WithField("projectId", projectID).Info("Getting project name from rk")

	if !r.IsDownloaded(projectID, projectName) {
		resp, err := http.DefaultClient.Get("http://" + r.rkHost + "/api/v1/projects/" + projectID + "/download")
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Expected error code 200 got %d", resp.StatusCode)
		}

		r.saveZip(projectID, projectName, resp.Body)
		resp.Body.Close()
	}
	return nil
}

func (r *RepositoryManager) GetProjectName(projectID string) (string, error) {
	resp, err := http.DefaultClient.Get("http://" + r.rkHost + "/api/v1/projects/" + projectID)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Expected error code 200 got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	project := &domain.Project{}
	err = json.Unmarshal(body, project)
	if err != nil {
		return "", err
	}
	return project.Name, nil
}

func (r *RepositoryManager) saveZip(projectID, projectName string, content io.ReadCloser) {
	r.l.WithField("projectID", projectID).Info("Saving project to storage")
	zipFile := r.store.ZipFilePath(projectID, projectName)
	err := r.store.Save(zipFile, content)
	if err != nil {
		r.l.WithFields(logrus.Fields{
			"projectID":   projectID,
			"projectName": projectName,
			"error":       err,
		}).Error("Unable to save zip")
		return
	}
}
