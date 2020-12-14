package service

// CheckoutCommit checks out the commit for a given project
func (r *RepositoryManager) CheckoutCommit(commit, projectID, projectName string) error {
	srcPath := r.store.UnzipPath(projectID)
	destPath := r.store.CommitPath(projectID, commit)
	err := r.store.Checkout(srcPath, destPath, commit, projectID, projectName)
	if err != nil {
		return err
	}
	return nil
}
