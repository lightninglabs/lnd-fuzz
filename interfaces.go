package lndfuzz

import (
	"io"
	"os"
	"os/exec"
)

// File represents a file handle.
type File interface {
	io.ReadWriteCloser
	io.ReaderFrom
}

// FileSystem provides an abstraction over file system operations.
type FileSystem interface {
	// ReadDir reads the directory and returns a list of directory entries.
	ReadDir(name string) ([]os.DirEntry, error)
	
	// Stat returns file info for the given path.
	Stat(name string) (os.FileInfo, error)
	
	// Open opens a file for reading.
	Open(name string) (File, error)
	
	// Create creates or truncates the named file.
	Create(name string) (File, error)
	
	// MkdirAll creates a directory path and all necessary parents.
	MkdirAll(path string, perm os.FileMode) error
	
	// Remove removes the named file or empty directory.
	Remove(name string) error
	
	// RemoveAll removes path and any children it contains.
	RemoveAll(path string) error
	
	// Rename renames (moves) a file.
	Rename(oldpath, newpath string) error
	
	// TempDir creates a new temporary directory.
	TempDir(dir, pattern string) (string, error)
}

// OSFileSystem implements FileSystem using the standard os package.
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

func (OSFileSystem) TempDir(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// CommandRunner provides an abstraction for running external commands.
type CommandRunner interface {
	// Run executes the command and returns its combined output.
	Run(dir string, env []string, name string, args ...string) ([]byte, error)
}

// CoverageProfileReader reads and parses coverage profiles.
type CoverageProfileReader interface {
	// ReadProfile reads a coverage profile from the given reader.
	ReadProfile(r io.Reader) (map[string]int, error)
}

// CoverageProfileWriter writes coverage profiles.
type CoverageProfileWriter interface {
	// WriteProfile writes a coverage profile to the given writer.
	WriteProfile(w io.Writer, blocks []CoverageBlock) error
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