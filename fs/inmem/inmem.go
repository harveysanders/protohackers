// Package inmem is an in-memory file system.
package inmem

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

// Check that *FS and *File implement the interfaces.
var _ fs.FS = (*FS)(nil)
var _ fs.File = (*File)(nil)
var _ fs.FileInfo = (*File)(nil)

var ErrInvalidArg = fmt.Errorf("invalid argument")
var ErrFileNotFound = fmt.Errorf("file not found")

type Entries map[string]*File

type FS struct {
	root Entries
	cwd  *File
}

// New returns a new in-memory file system.
func New() *FS {
	rootFile := &File{
		name:       "/",
		isDir:      true,
		modifiedAt: time.Now(),
		files:      make(Entries, 10),
	}
	root := make(Entries, 10)
	root["/"] = rootFile
	return &FS{
		root: root,
		cwd:  rootFile,
	}
}

// Open opens the named file.
func (f *FS) Open(name string) (fs.File, error) {
	if name == "" {
		return nil, ErrInvalidArg
	}

	if name == "." {
		return f.cwd, nil
	}

	if name == "/" {
		return f.root["/"], nil
	}

	dirs, filename := SplitPath(name)

	// Recursively traverse the map
	file, err := f.open(dirs, filename, f.root)
	if err != nil {
		if err == ErrFileNotFound {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return nil, err
	}

	return file, nil
}

func (f *FS) open(dirnames []string, fileName string, root Entries) (*File, error) {
	if len(dirnames) == 0 {
		dirnames = []string{f.cwd.name}
	}

	if len(dirnames) == 1 {
		if file, ok := root[dirnames[0]]; ok {
			if fileName == "" {
				if !file.isDir {
					panic("file is not a directory")
				}
				return file, nil
			}
			if file, ok := file.files[fileName]; ok {
				return file, nil
			}
		}
		return nil, ErrFileNotFound
	}
	nextDir := dirnames[0]
	if file, ok := root[nextDir]; ok {
		if !file.isDir {
			return nil, ErrFileNotFound
		}
		return f.open(dirnames[1:], fileName, file.files)
	}
	return nil, ErrFileNotFound
}

// MkdirAll creates a directory named path, along with any necessary parents.
func (f *FS) MkdirAll(name string, perm fs.FileMode) error {
	if name == "" {
		return ErrInvalidArg
	}

	if name == "." {
		return nil
	}

	dirs, filename := SplitPath(name)
	// Recursively directory creation traverse the map
	return f.mkdir(dirs, filename, f.root)
}

func (f *FS) mkdir(dirnames []string, filename string, root Entries) error {
	if len(dirnames) == 0 {
		if filename == "" {
			return ErrInvalidArg
		}

		dir, ok := root[f.cwd.name]
		if !ok {
			panic("current directory not found")
		}

		dir.files[filename] = &File{
			name:       filename,
			isDir:      true,
			modifiedAt: time.Now(),
			files:      make(Entries, 0),
		}
		return nil
	}

	if len(dirnames) == 1 {
		if _, ok := root[dirnames[0]]; ok {
			return nil
		}
		nextDir := dirnames[0]
		if nextDir == "" && filename != "" {
			nextDir = filename
		}
		root[nextDir] = &File{
			name:       nextDir,
			isDir:      true,
			modifiedAt: time.Now(),
			files:      make(Entries, 0),
		}
		return nil
	}

	nextDir := dirnames[0]
	if _, ok := root[nextDir]; !ok {
		root[nextDir] = &File{
			name:       nextDir,
			isDir:      true,
			modifiedAt: time.Now(),
			files:      make(Entries, 10),
		}
	}
	return f.mkdir(dirnames[1:], filename, root[nextDir].files)
}

func (f *FS) Create(name string) (*File, error) {
	if err := f.WriteFile(name, nil, 0); err != nil {
		return nil, err
	}
	newFile, err := f.Open(name)
	file, ok := newFile.(*File)
	if !ok {
		return nil, fmt.Errorf("not a File")
	}
	return file, err
}

func (f *FS) OpenFile(name string, flag int, perm os.FileMode) (*File, error) {
	opened, err := f.Open(name)
	if err != nil {
		// If not creating the file, return the error
		if flag&os.O_CREATE == 0 {
			return nil, err
		}
		// In create mode
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			return nil, err
		}
		// Create the file if it doesn't exist
		opened, err = f.Create(name)
		if err != nil {
			return nil, err
		}
	}

	file, ok := opened.(*File)
	if !ok {
		return nil, fmt.Errorf("not a File")
	}
	file.appendMode = flag&os.O_APPEND != 0
	return file, nil
}

// WriteFile writes data to a file named by filename.
func (f *FS) WriteFile(filePath string, data []byte, perm fs.FileMode) error {
	if filePath == "" {
		return ErrInvalidArg
	}

	if filePath == "." {
		return nil
	}

	dirs, fileName := SplitPath(filePath)
	// Recursively directory creation traverse the map
	newRoot, err := writeFile(dirs, fileName, data, f.root)
	if err != nil {
		return err
	}
	f.root = newRoot
	return nil
}

// SplitPath splits a file path into its directory and file name components.
// The directory path is returned as a slice of strings, where each element is a directory name.
func SplitPath(filePath string) ([]string, string) {
	sep := string(os.PathSeparator)
	dirPath, fileName := path.Split(filePath)
	out := strings.Split(dirPath, sep)
	if strings.HasPrefix(filePath, sep) {
		out[0] = sep
	}

	// Remove the extra empty string at the end
	if len(out) > 0 {
		return out[:len(out)-1], fileName
	}
	return out, fileName
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	d, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	dir, ok := d.(*File)
	if !ok {
		return nil, fmt.Errorf("not a File")
	}
	if !dir.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	entries := make([]fs.DirEntry, 0, len(dir.files))
	for _, v := range dir.files {
		entries = append(entries, v)
	}
	return entries, nil
}

func writeFile(dirnames []string, filename string, data []byte, root Entries) (Entries, error) {
	if len(dirnames) == 0 {
		return root, ErrInvalidArg
	}

	if len(dirnames) == 1 {
		finalDir := dirnames[0]
		if _, ok := root[finalDir]; !ok {
			// Create the directory
			root[finalDir] = &File{
				name:       finalDir,
				isDir:      true,
				modifiedAt: time.Now(),
				files:      make(Entries, 0),
			}
		}

		// Create the file
		contents := make([]byte, len(data))
		copy(contents, data)
		root[finalDir].files[filename] = &File{
			name:       filename,
			contents:   contents,
			modifiedAt: time.Now(),
		}
		return root, nil
	}

	nextDir := dirnames[0]
	if _, ok := root[nextDir]; !ok {
		root[nextDir] = &File{
			name:       nextDir,
			isDir:      true,
			modifiedAt: time.Now(),
			files:      make(Entries, 10),
		}
	}
	files, err := writeFile(dirnames[1:], filename, data, root[nextDir].files)
	root[nextDir].files = files
	return root, err
}

type File struct {
	name string
	// Contents of the file. Is nil if the file is a directory.
	contents   []byte
	rdr        *bufio.Reader
	isDir      bool
	modifiedAt time.Time
	// Files in the directory. Is nil if the file is not a directory. The keys are the file names.
	files      Entries
	appendMode bool
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.rdr == nil {
		f.rdr = bufio.NewReader(strings.NewReader(string(f.contents)))
	}
	return f.rdr.Read(p)
}

func (f *File) Write(data []byte) (int, error) {
	if f.appendMode {
		f.contents = append(f.contents, data...)
		return len(data), nil
	}
	return copy(f.contents, data), nil
}

func (f *File) Close() error {
	f.rdr.Reset(bytes.NewReader(f.contents))
	return nil
}

func (f *File) Stat() (fs.FileInfo, error) {
	return f, nil
}

func (f *File) IsDir() bool {
	return f.isDir
}

func (f *File) Name() string {
	return f.name
}

func (f *File) Size() int64 {
	return int64(len(f.contents))
}

func (f *File) Mode() fs.FileMode {
	return 0
}

func (f *File) ModTime() time.Time {
	return f.modifiedAt
}

func (f *File) Sys() any {
	return nil
}

// Info implements fs.DirEntry.
func (f *File) Info() (fs.FileInfo, error) {
	return f, nil
}

func (f *File) Type() fs.FileMode {
	return f.Mode()
}
