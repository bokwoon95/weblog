package renderly

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func AbsDir(relativePath string) string {
	_, absolutePath, _, _ := runtime.Caller(1)
	return filepath.Join(absolutePath, relativePath) + string(os.PathSeparator)
}

// fs.FS
type FS interface {
	Open(name string) (File, error)
}

// fs.File
type File interface {
	Stat() (os.FileInfo, error)
	Read([]byte) (int, error)
	Close() error
}

// fs.ValidPath
func validPath(name string) bool {
	if name == "." {
		return true
	}
	for {
		i := 0
		for i < len(name) && name[i] != '/' {
			if name[i] == '\\' {
				return false
			}
			i++
		}
		elem := name[:i]
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
		if i == len(name) {
			return true
		}
		name = name[i+1:]
	}
}

// os.DirFS
func DirFS(dir string) FS {
	return dirFS(dir)
}

// os.dirFS
type dirFS string

// (os.dirFS).Open
func (dir dirFS) Open(name string) (File, error) {
	if !validPath(name) {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrInvalid}
	}
	f, err := os.Open(string(dir) + string(os.PathSeparator) + name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// fs.ReadFile
func ReadFile(fsys FS, name string) ([]byte, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var size int
	if info, err := file.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}
	data := make([]byte, 0, size+1)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := file.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}
