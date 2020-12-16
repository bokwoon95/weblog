package renderx

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/bokwoon95/weblog/renderx/fs"
)

type File struct {
	FS   fs.FS
	Name string
}

func (c File) ReadString() (string, error) {
	b, err := fs.ReadFile(c.FS, c.Name)
	return string(b), err
}

func AbsDir(skip int) string {
	_, filename, _, _ := runtime.Caller(1)
	elems := []string{filepath.Dir(filename)}
	for i := 0; i < skip; i++ {
		elems = append(elems, "..")
	}
	return filepath.Join(elems...) + string(os.PathSeparator)
}
