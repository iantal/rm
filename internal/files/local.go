package files

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/iantal/rm/internal/util"
	"golang.org/x/xerrors"
)

// Local is an implementation of the Storage interface which works with the
// local disk on the current machine
type Local struct {
	log         *util.StandardLogger
	maxFileSize int // maximum numbber of bytes for files
	basePath    string
}

// NewLocal creates a new Local filesytem with the given base path
// basePath is the base directory to save files to
// maxSize is the max number of bytes that a file can be
func NewLocal(l *util.StandardLogger, basePath string, maxSize int) (*Local, error) {
	p, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	return &Local{l, maxSize, p}, nil
}

// Save the contents of the Writer to the given path
// path is a relative path, basePath will be appended
func (l *Local) Save(fp string, contents io.Reader) error {
	// get the directory and make sure it exists
	d := filepath.Dir(fp)
	err := os.MkdirAll(d, os.ModePerm)
	if err != nil {
		return xerrors.Errorf("Unable to create directory: %w", err)
	}

	// if the file exists delete it
	_, err = os.Stat(fp)
	if err == nil {
		err = os.Remove(fp)
		if err != nil {
			return xerrors.Errorf("Unable to delete file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// if this is anything other than a not exists error
		return xerrors.Errorf("Unable to get file info: %w", err)
	}

	// create a new file at the path
	f, err := os.Create(fp)
	if err != nil {
		return xerrors.Errorf("Unable to create file: %w", err)
	}
	defer f.Close()

	// write the contents to the new file
	// ensure that we are not writing greater than max bytes
	_, err = io.Copy(f, contents)
	if err != nil {
		return xerrors.Errorf("Unable to write to file: %w", err)
	}

	return nil
}

// Get the file at the given path and return a Reader
// the calling function is responsible for closing the reader
func (l *Local) Get(path string) (*os.File, error) {
	// get the full path for the file
	fp := l.FullPath(path)

	// open the file
	f, err := os.Open(fp)
	if err != nil {
		return nil, xerrors.Errorf("Unable to open file: %w", err)
	}

	return f, nil
}

// FullPath returns the absolute path
func (l *Local) FullPath(path string) string {
	// append the given path to the base path
	return filepath.Join(l.basePath, path)
}

// Unzip uses the unzip command line tool to extract the project to the specified target directory
func (l *Local) Unzip(archive, target, name string) error {
	td := filepath.Join(target, name)
	if err := os.MkdirAll(td, 0755); err != nil {
		return xerrors.Errorf("Unable to create target directory: %w", err)
	}

	cmd := exec.Command("unzip", "-qq", archive, "-d", td)

	if err := cmd.Run(); err != nil {
		return xerrors.Errorf("Unable to unzip archive: %w", err)
	}

	cmd.ProcessState.Sys()
	return nil
}

func (l *Local) Checkout(src, dest, commit, projectID, name string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return xerrors.Errorf("Unable to create target directory: %w", err)
	}

	os.Chdir(filepath.Join(src, name))

	// checkout the commit
	cmd := exec.Command("git", "checkout", commit)
	err := runCmd(cmd)
	if err != nil {
		return xerrors.Errorf("Git checkout error: %w", err)
	}

	// bundle the commit
	bf := name + ".bundle"
	cmd = exec.Command("git", "bundle", "create", filepath.Join(dest, bf), "HEAD")
	err = runCmd(cmd)
	if err != nil {
		return xerrors.Errorf("Git bundle error: %w", err)
	}

	// reset
	cmd = exec.Command("git", "reset", "--hard")
	err = runCmd(cmd)
	if err != nil {
		return xerrors.Errorf("Git reset error: %w", err)
	}

	os.Chdir("-")
	return nil

}

func runCmd(cmd *exec.Cmd) error {
	if err := cmd.Run(); err != nil {
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			exitError.Sys()
		}
		return err
	}

	cmd.ProcessState.Sys()
	return nil
}
