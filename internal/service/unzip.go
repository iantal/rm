package service

import (
	"github.com/sirupsen/logrus"
)

func (r *RepositoryManager) ExtractZip(zipFile, projectID, projectName string) (string, error) {
	r.l.WithFields(logrus.Fields{
		"projectID": projectID,
		"ZipFile":   zipFile,
	}).Info("Unzipping")

	unzipPath := r.store.UnzipPath(projectID)
	err := r.store.Unzip(zipFile, unzipPath, projectName)
	if err != nil {
		return "", err
	}

	return unzipPath, nil
}
