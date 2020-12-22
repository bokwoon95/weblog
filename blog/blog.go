package blog

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/erro"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
	"github.com/dgraph-io/ristretto"
	"github.com/go-chi/chi"
)

type Blog struct {
	*pagemanager.PageManager
	namespace string // URL prefix
	render    *renderly.Renderly
	cache     *ristretto.Cache
}

var builtin = os.DirFS(renderly.AbsDir("."))

func New(namespace string) func(*pagemanager.PageManager) (pagemanager.Plugin, error) {
	return func(pm *pagemanager.PageManager) (pagemanager.Plugin, error) {
		var err error
		blg := &Blog{
			PageManager: pm,
		}
		templatesDir := os.DirFS("./templates/plainsimple")
		blg.render, err = renderly.New(
			builtin,
			renderly.GlobalCSS(builtin, "tachyons.css", "style.css"),
			renderly.AltFS("templates", templatesDir),
		)
		if err != nil {
			return blg, erro.Wrap(err)
		}
		blg.cache, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: 1e7,     // number of keys to track frequency of (10M).
			MaxCost:     1 << 30, // maximum cost of cache (1GB).
			BufferItems: 64,      // number of keys per Get buffer.
			Metrics:     true,
		})
		if err != nil {
			return blg, err
		}
		return blg, nil
	}
}

func (blg *Blog) kvGet(key string) (sql.NullString, error) {
	data, found := blg.cache.Get(key)
	value, ok := data.(sql.NullString)
	if found && ok {
		return value, nil
	}
	query := "SELECT value FROM blg_kv WHERE key = ?"
	err := blg.DB.QueryRow(query, key).Scan(&value)
	if err != nil {
		return value, err
	}
	_ = blg.cache.Set(key, value, 0)
	return value, nil
}

func (blg *Blog) kvSet(key, value string) error {
	query := "INSERT INTO blg_kv (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value"
	_, err := blg.DB.Exec(query, key, value)
	if err != nil {
		return err
	}
	_ = blg.cache.Set(key, value, 0)
	return nil
}

func (blg *Blog) AddRoutes() error {
	blg.Router.Route("/blog", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			err := blg.render.Page(w, r, nil, "blog.html")
			if err != nil {
				blg.render.InternalServerError(w, r, err)
				return
			}
		})
		r.Get("/edit", func(w http.ResponseWriter, r *http.Request) {
			err := blg.render.Page(w, r, nil, "blog.html", "edit_mode.css", "edit_mode.js")
			if err != nil {
				blg.render.InternalServerError(w, r, err)
				return
			}
		})
	})
	return nil
}

const (
	configPostIndex = "post-index"
	configpost      = "post"
)

func (blg *Blog) postIndexFilenames() ([]string, error) {
	var templateName string
	value, err := blg.kvGet(configPostIndex)
	if err != nil {
		return nil, err
	}
	if value.Valid {
		templateName = value.String
	} else {
		templateName = "blog.html" // default template
	}
	var filenames = resolve(templateName)
	return filenames, nil
}

func resolve(name string) []string {
	return nil
}
