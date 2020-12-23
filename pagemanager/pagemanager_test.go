package pagemanager

import (
	"os"
	"testing"

	"github.com/bokwoon95/weblog/pagemanager/renderly"
	"github.com/davecgh/go-spew/spew"
	"github.com/matryer/is"
)

func Test_Uh(t *testing.T) {
	is := is.New(t)
	fsys := os.DirFS(renderly.AbsDir("../themes"))
	src, err := getPageSource(fsys, "plainsimple/wawa.html")
	is.NoErr(err)
	spew.Dump(src)
}
