// Package inmem is an in-memory file system.
package inmem

import (
	"bufio"
	"bytes"
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
	root map[string]*File
	cwd  *File
}

// New returns a new in-memory file system.
func New() *FS {
	return &FS{
		root: make(Entries, 10),
		cwd: &File{
			name:       "/",
			isDir:      true,
			modifiedAt: time.Now(),
			files:      make(Entries, 10),
		},
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

	path, filename := path.Split(name)
	dirs := strings.Split(strings.TrimSuffix(path, string(os.PathSeparator)), string(os.PathSeparator))

	// Recursively traverse the map
	file, err := open(dirs, filename, f.root)
	if err != nil {
		if err == ErrFileNotFound {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return nil, err
	}

	return file, nil
}

func open(dirnames []string, fileName string, root Entries) (*File, error) {
	if len(dirnames) == 0 {
		return nil, ErrInvalidArg
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
		return open(dirnames[1:], fileName, file.files)
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

	path, filename := path.Split(name)
	dirs := strings.Split(strings.TrimSuffix(path, string(os.PathSeparator)), string(os.PathSeparator))
	// Recursively directory creation traverse the map
	return mkdir(dirs, filename, f.root)
}

func mkdir(dirnames []string, filename string, root Entries) error {
	if len(dirnames) == 0 {
		return ErrInvalidArg
	}

	if len(dirnames) == 1 {
		if _, ok := root[dirnames[0]]; ok {
			return nil
		}
		nextDir := dirnames[0]
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
	return mkdir(dirnames[1:], filename, root[nextDir].files)
}

// WriteFile writes data to a file named by filename.
func (f *FS) WriteFile(filePath string, data []byte, perm fs.FileMode) error {
	if filePath == "" {
		return ErrInvalidArg
	}

	if filePath == "." {
		return nil
	}

	dirPath, fileName := path.Split(filePath)
	dirs := strings.Split(strings.TrimSuffix(dirPath, string(os.PathSeparator)), string(os.PathSeparator))

	// Recursively directory creation traverse the map
	newRoot, err := writeFile(dirs, fileName, data, f.root)
	if err != nil {
		return err
	}
	f.root = newRoot
	return nil
}

func writeFile(dirnames []string, filename string, data []byte, root Entries) (Entries, error) {
	if len(dirnames) == 0 {
		return root, ErrInvalidArg
	}

	if len(dirnames) == 1 {
		lastDir := dirnames[0]
		if _, ok := root[lastDir]; !ok {
			// Create the directory
			root[lastDir] = &File{
				name:       lastDir,
				isDir:      true,
				modifiedAt: time.Now(),
				files:      make(Entries, 0),
			}
		}

		// Create the file
		contents := make([]byte, len(data))
		copy(contents, data)
		root[lastDir].files[filename] = &File{
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
	return writeFile(dirnames[1:], filename, data, root[nextDir].files)
}

type File struct {
	name string
	// Contents of the file. Is nil if the file is a directory.
	contents   []byte
	rdr        *bufio.Reader
	isDir      bool
	modifiedAt time.Time
	// Files in the directory. Is nil if the file is not a directory. The keys are the file names.
	files Entries
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.rdr == nil {
		f.rdr = bufio.NewReader(strings.NewReader(string(f.contents)))
	}
	return f.rdr.Read(p)
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
