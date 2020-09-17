package files

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/xerrors"
)

// Local is an implementation of the Storage interface which works with the
// local disk on the current machine
type Local struct {
	maxFileSize int // maximum numbber of bytes for files
	basePath    string
}

// NewLocal creates a new Local filesytem with the given base path
// basePath is the base directory to save files to
// maxSize is the max number of bytes that a file can be
func NewLocal(basePath string, maxSize int) (*Local, error) {
	p, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	return &Local{basePath: p}, nil
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

	var waitStatus syscall.WaitStatus
	if err := cmd.Run(); err != nil {
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			fmt.Printf("Output: %s\n", []byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
		return err
	}

	waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)

	return nil
}

// Zip uses the zip command line tool to compress the project to the specified target directory
func (l *Local) Zip(src, dest, dir, name string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return xerrors.Errorf("Unable to create target directory: %w", err)
	}

	archive := name + ".zip"

	os.Chdir(filepath.Join(src, name))
	cmd := exec.Command("zip", filepath.Join(dest, archive), "-r", dir)
	os.Chdir("-")

	var waitStatus syscall.WaitStatus
	if err := cmd.Run(); err != nil {
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			fmt.Printf("Output: %s\n", []byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
		return err
	}

	waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)

	return nil
}

func (l *Local) Checkout(src, dest, commit, name string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return xerrors.Errorf("Unable to create target directory: %w", err)
	}

	fmt.Printf("Changing dir to %s\n", filepath.Join(src, name))
	os.Chdir(filepath.Join(src, name))

	// checkout the commit
	cmd := exec.Command("git", "checkout", commit)
	err := runCmd(cmd)
	if err != nil {
		fmt.Print("Checkout error")
		return err
	}

	// bundle the commit
	bf := name + ".bundle"
	cmd = exec.Command("git", "bundle", "create", filepath.Join(dest, bf), "HEAD")
	err = runCmd(cmd)
	if err != nil {
		fmt.Print("Bundle error")
		return err
	}

	// reset
	cmd = exec.Command("git", "reset", "--hard")
	err = runCmd(cmd)
	if err != nil {
		fmt.Print("Reset error")
		return err
	}

	os.Chdir("-")
	return nil

}

func runCmd(cmd *exec.Cmd) error {
	var waitStatus syscall.WaitStatus
	if err := cmd.Run(); err != nil {
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			fmt.Printf("Output checkout: %s\n", []byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
		return err
	}

	waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
	return nil
}
