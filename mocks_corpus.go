package lndfuzz

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MockFileSystem is a mock implementation of FileSystem for testing.
type MockFileSystem struct {
	mu      sync.Mutex
	files   map[string]*mockFile
	dirs    map[string]bool
	tempNum int
}

type mockFile struct {
	name    string
	content []byte
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

// NewMockFileSystem creates a new mock file system.
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string]*mockFile),
		dirs:  make(map[string]bool),
	}
}

// AddFile adds a file to the mock filesystem.
func (m *MockFileSystem) AddFile(path string, content []byte, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if size == 0 && content != nil {
		size = int64(len(content))
	}
	
	m.files[path] = &mockFile{
		name:    filepath.Base(path),
		content: content,
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}
	
	// Ensure parent directories exist
	dir := filepath.Dir(path)
	for dir != "." && dir != "/" {
		m.dirs[dir] = true
		dir = filepath.Dir(dir)
	}
}

// AddDir adds a directory to the mock filesystem.
func (m *MockFileSystem) AddDir(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[path] = true
}

// FileExists checks if a file exists in the mock filesystem.
func (m *MockFileSystem) FileExists(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.files[path]
	return exists
}

// DirExists checks if a directory exists in the mock filesystem.
func (m *MockFileSystem) DirExists(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dirs[path]
}

// GetFileContent returns the content of a file.
func (m *MockFileSystem) GetFileContent(path string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.files[path]; ok {
		return f.content
	}
	return nil
}

// ReadDir implements FileSystem.ReadDir
func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var entries []os.DirEntry
	
	// Add files in this directory
	for path, file := range m.files {
		if filepath.Dir(path) == name {
			entries = append(entries, &mockDirEntry{
				name:  file.name,
				isDir: false,
				info:  file,
			})
		}
	}
	
	// Add subdirectories
	for dir := range m.dirs {
		if filepath.Dir(dir) == name {
			entries = append(entries, &mockDirEntry{
				name:  filepath.Base(dir),
				isDir: true,
				info: &mockFile{
					name:  filepath.Base(dir),
					isDir: true,
					mode:  0755,
				},
			})
		}
	}
	
	if len(entries) == 0 && !m.dirs[name] {
		return nil, os.ErrNotExist
	}
	
	return entries, nil
}

// Stat implements FileSystem.Stat
func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if file, ok := m.files[name]; ok {
		return file, nil
	}
	if m.dirs[name] {
		return &mockFile{
			name:  filepath.Base(name),
			isDir: true,
			mode:  0755,
		}, nil
	}
	return nil, os.ErrNotExist
}

// Open implements FileSystem.Open
func (m *MockFileSystem) Open(name string) (File, error) {
	m.mu.Lock()
	file, ok := m.files[name]
	m.mu.Unlock()
	
	if !ok {
		return nil, os.ErrNotExist
	}
	
	// Create a temporary file with the content
	tmpFile, err := os.CreateTemp("", "mock-*")
	if err != nil {
		return nil, err
	}
	
	if _, err := tmpFile.Write(file.content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}
	
	// Seek back to beginning
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}
	
	return tmpFile, nil
}

// Create implements FileSystem.Create
func (m *MockFileSystem) Create(name string) (File, error) {
	m.mu.Lock()
	
	m.files[name] = &mockFile{
		name:    filepath.Base(name),
		content: []byte{},
		mode:    0644,
		modTime: time.Now(),
	}
	
	// Ensure parent directories exist
	dir := filepath.Dir(name)
	for dir != "." && dir != "/" {
		m.dirs[dir] = true
		dir = filepath.Dir(dir)
	}
	m.mu.Unlock()
	
	// Create a temporary file that we'll track
	tmpFile, err := os.CreateTemp("", "mock-create-*")
	if err != nil {
		return nil, err
	}
	
	// Wrap the file to intercept writes
	return &mockFileHandle{
		File: tmpFile,
		fs:   m,
		path: name,
	}, nil
}

// MkdirAll implements FileSystem.MkdirAll
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	parts := strings.Split(path, string(filepath.Separator))
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		m.dirs[current] = true
	}
	return nil
}

// Remove implements FileSystem.Remove
func (m *MockFileSystem) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.files[name]; ok {
		delete(m.files, name)
		return nil
	}
	if m.dirs[name] {
		delete(m.dirs, name)
		return nil
	}
	return os.ErrNotExist
}

// RemoveAll implements FileSystem.RemoveAll
func (m *MockFileSystem) RemoveAll(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Remove all files and dirs with this prefix
	for p := range m.files {
		if strings.HasPrefix(p, path) {
			delete(m.files, p)
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, path) {
			delete(m.dirs, d)
		}
	}
	return nil
}

// Rename implements FileSystem.Rename
func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if file, ok := m.files[oldpath]; ok {
		m.files[newpath] = file
		delete(m.files, oldpath)
		return nil
	}
	if m.dirs[oldpath] {
		m.dirs[newpath] = true
		delete(m.dirs, oldpath)
		// Move all children
		for p, f := range m.files {
			if strings.HasPrefix(p, oldpath+string(filepath.Separator)) {
				newP := strings.Replace(p, oldpath, newpath, 1)
				m.files[newP] = f
				delete(m.files, p)
			}
		}
		return nil
	}
	return os.ErrNotExist
}

// mockFile implements os.FileInfo
func (f *mockFile) Name() string       { return f.name }
func (f *mockFile) Size() int64        { return int64(len(f.content)) }
func (f *mockFile) Mode() os.FileMode  { return f.mode }
func (f *mockFile) ModTime() time.Time { return f.modTime }
func (f *mockFile) IsDir() bool        { return f.isDir }
func (f *mockFile) Sys() interface{}   { return nil }

// mockDirEntry implements os.DirEntry
type mockDirEntry struct {
	name  string
	isDir bool
	info  os.FileInfo
}

func (d *mockDirEntry) Name() string               { return d.name }
func (d *mockDirEntry) IsDir() bool                { return d.isDir }
func (d *mockDirEntry) Type() os.FileMode          { return d.info.Mode().Type() }
func (d *mockDirEntry) Info() (os.FileInfo, error) { return d.info, nil }

// mockFileHandle wraps an os.File to intercept operations
type mockFileHandle struct {
	*os.File
	fs   *MockFileSystem
	path string
}

// Write overrides Write to update the mock filesystem
func (h *mockFileHandle) Write(p []byte) (n int, err error) {
	n, err = h.File.Write(p)
	if err == nil {
		h.updateContent()
	}
	return n, err
}

// Close overrides Close to update content and clean up
func (h *mockFileHandle) Close() error {
	h.updateContent()
	err := h.File.Close()
	// Clean up temp file
	os.Remove(h.File.Name())
	return err
}

// ReadFrom implements io.ReaderFrom
func (h *mockFileHandle) ReadFrom(r io.Reader) (n int64, err error) {
	n, err = io.Copy(h.File, r)
	if err == nil {
		h.updateContent()
	}
	return n, err
}

// updateContent reads the temp file and updates the mock filesystem
func (h *mockFileHandle) updateContent() {
	// Seek to beginning
	h.File.Seek(0, 0)
	content, _ := io.ReadAll(h.File)
	
	h.fs.mu.Lock()
	defer h.fs.mu.Unlock()
	
	if file, ok := h.fs.files[h.path]; ok {
		file.content = content
	}
}

// MockCommandRunner is a mock implementation of CommandRunner for testing.
type MockCommandRunner struct {
	mu       sync.Mutex
	commands []MockCommand
	outputs  map[string]MockOutput
}

type MockCommand struct {
	Dir  string
	Name string
	Args []string
	Env  []string
}

type MockOutput struct {
	Output []byte
	Error  error
}

// NewMockCommandRunner creates a new mock command runner.
func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		outputs: make(map[string]MockOutput),
	}
}

// SetOutput sets the output for a specific command.
func (m *MockCommandRunner) SetOutput(name string, args []string, output []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.commandKey(name, args)
	m.outputs[key] = MockOutput{
		Output: output,
		Error:  err,
	}
}

// GetCommands returns all commands that were run.
func (m *MockCommandRunner) GetCommands() []MockCommand {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return append([]MockCommand{}, m.commands...)
}

// Run implements CommandRunner.Run
func (m *MockCommandRunner) Run(dir string, env []string, name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.commands = append(m.commands, MockCommand{
		Dir:  dir,
		Name: name,
		Args: args,
		Env:  env,
	})
	
	key := m.commandKey(name, args)
	if output, ok := m.outputs[key]; ok {
		return output.Output, output.Error
	}
	
	// Default output for go test commands
	if name == "go" && len(args) > 0 && args[0] == "test" {
		return []byte("DEBUG finished processing ... initial coverage bits: 100"), nil
	}
	
	return nil, fmt.Errorf("unexpected command: %s %v", name, args)
}

func (m *MockCommandRunner) commandKey(name string, args []string) string {
	return fmt.Sprintf("%s %s", name, strings.Join(args, " "))
}