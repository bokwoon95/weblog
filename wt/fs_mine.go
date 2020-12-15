package wt

import (
	"os"
	"path/filepath"
	"runtime"
)

type NamedFS struct {
	FS
	Name string
}

type Fyle struct {
	FS   NamedFS
	Name string
}

func (c Fyle) ReadString() (string, error) {
	b, err := ReadFile(c.FS, c.Name)
	return string(b), err
}

func (c Fyle) FullName() string {
	return c.FS.Name + string(os.PathSeparator) + c.Name
}

func AbsDir(skip int) string {
	_, filename, _, _ := runtime.Caller(1)
	elems := []string{filepath.Dir(filename)}
	for i := 0; i < skip; i++ {
		elems = append(elems, "..")
	}
	return filepath.Join(elems...) + string(os.PathSeparator)
}
