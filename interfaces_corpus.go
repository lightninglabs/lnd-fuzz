package lndfuzz

import (
	"io"
	"os"
)

// FileSystem is an abstraction over filesystem operations.
type FileSystem interface {
	// ReadDir reads the directory and returns a list of directory entries.
	ReadDir(name string) ([]os.DirEntry, error)
	
	// Stat returns file info for the given path.
	Stat(name string) (os.FileInfo, error)
	
	// Open opens the named file for reading.
	Open(name string) (File, error)
	
	// Create creates or truncates the named file.
	Create(name string) (File, error)
	
	// MkdirAll creates a directory path.
	MkdirAll(path string, perm os.FileMode) error
	
	// Remove removes the named file or empty directory.
	Remove(name string) error
	
	// RemoveAll removes path and any children it contains.
	RemoveAll(path string) error
	
	// Rename renames a file or directory.
	Rename(oldpath, newpath string) error
}

// File represents an open file.
type File interface {
	io.ReadWriteCloser
	io.ReaderFrom
	Name() string
}

// CommandRunner executes external commands.
type CommandRunner interface {
	// Run executes the command and returns its combined output.
	Run(dir string, env []string, name string, args ...string) ([]byte, error)
}

// ProgressReporter is an interface for reporting progress during long operations.
type ProgressReporter interface {
	// ReportProgress is called periodically during operations.
	ReportProgress(current, total int, message string)
	
	// ReportInfo reports general information messages.
	ReportInfo(message string)
	
	// ReportWarning reports warning messages.
	ReportWarning(message string)
	
	// ReportError reports error messages that don't stop execution.
	ReportError(message string)
}