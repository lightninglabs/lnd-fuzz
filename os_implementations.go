package lndfuzz

import (
	"os"
	"os/exec"
)

// OSFileSystem implements FileSystem using the real filesystem.
type OSFileSystem struct{}

func (OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (OSFileSystem) Open(name string) (File, error) {
	return os.Open(name)
}

func (OSFileSystem) Create(name string) (File, error) {
	return os.Create(name)
}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// OSCommandRunner implements CommandRunner using os/exec.
type OSCommandRunner struct{}

// Run executes the command and returns its combined output.
func (OSCommandRunner) Run(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd.CombinedOutput()
}

// NullProgressReporter is a no-op implementation of ProgressReporter.
type NullProgressReporter struct{}

func (NullProgressReporter) ReportProgress(current, total int, message string) {}
func (NullProgressReporter) ReportInfo(message string)                         {}
func (NullProgressReporter) ReportWarning(message string)                      {}
func (NullProgressReporter) ReportError(message string)                        {}