package blog

import (
	"net/http"
	"os"

	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/pagemanager/erro"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
)

type Blog struct {
	*pagemanager.PageManager
	namespace string // URL prefix
	Render    *renderly.Renderly
}

var builtin = os.DirFS(renderly.AbsDir("."))

func New(namespace string) func(*pagemanager.PageManager) (pagemanager.Plugin, error) {
	return func(pm *pagemanager.PageManager) (pagemanager.Plugin, error) {
		var err error
		blg := &Blog{
			PageManager: pm,
		}
		templatesDir := os.DirFS(pm.RootDirectory + "templates")
		blg.Render, err = renderly.New(
			templatesDir,
			renderly.GlobalCSS(builtin, "tachyons.css", "style.css"),
			renderly.AltFS("builtin", builtin),
		)
		if err != nil {
			return blg, erro.Wrap(err)
		}
		return blg, nil
	}
}

func (blg *Blog) AddRoutes() error {
	blg.Router.Route("/blog", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			err := blg.Render.Page(w, r, nil, "blog.html?fs=builtin")
			if err != nil {
				blg.Render.InternalServerError(w, r, err)
				return
			}
		})
		r.Get("/edit", func(w http.ResponseWriter, r *http.Request) {
			err := blg.Render.Page(w, r, nil, "blog.html?fs=builtin", "edit_mode.css?fs=builtin", "edit_mode.js?fs=builtin")
			if err != nil {
				blg.Render.InternalServerError(w, r, err)
				return
			}
		})
	})
	return nil
}
