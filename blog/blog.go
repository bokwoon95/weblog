package blog

import (
	"net/http"
	"os"

	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/erro"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
)

type Blog struct {
	*pagemanager.PageManager
	namespace string // URL prefix
	Render    *renderly.Renderly
}

func New(namespace string) func(*pagemanager.PageManager) (pagemanager.Plugin, error) {
	return func(manager *pagemanager.PageManager) (pagemanager.Plugin, error) {
		var err error
		blg := &Blog{
			PageManager: manager,
		}
		fsys := os.DirFS(renderly.AbsDir("."))
		blg.Render, err = renderly.New(
			fsys,
			renderly.GlobalCSS(fsys, "tachyons.css"),
		)
		if err != nil {
			return blg, erro.Wrap(err)
		}
		return blg, nil
	}
}

func (blg *Blog) AddRoutes() error {
	blg.Router.Get("/blog", func(w http.ResponseWriter, r *http.Request) {
		err := blg.Render.Page(w, r, nil, "blog.html", "blog.js")
		if err != nil {
			blg.Render.InternalServerError(w, r, err)
			return
		}
	})
	return nil
}
