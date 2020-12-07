package pagemanager

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/pagemanager/chi/middleware"
	"github.com/dgraph-io/ristretto"
	_ "github.com/mattn/go-sqlite3"
)

const (
	pageKey     = "pageKey\x00"
	redirectKey = "redirectKey\x00"
	handlerKey  = "handlerKey\x00"
	disabledKey = "disabledKey\x00"
)

type PageManager struct {
	Restart chan struct{}
	DB      *sql.DB
	Cache   *ristretto.Cache
	Router  *chi.Mux
}

func New(driverName, dataSourceName string) (*PageManager, error) {
	var err error
	pm := &PageManager{}
	// Restart
	pm.Restart = make(chan struct{}, 1)
	// DB
	pm.DB, err = sql.Open(driverName, dataSourceName)
	if err != nil {
		return pm, err
	}
	if driverName == "sqlite3" {
		_, err = pm.DB.Exec("PRAGMA journal_mode = WAL")
		if err != nil {
			return pm, err
		}
		_, err = pm.DB.Exec("PRAGMA synchronous = normal")
		if err != nil {
			return pm, err
		}
		_, err = pm.DB.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			return pm, err
		}
	}
	err = pm.DB.Ping()
	if err != nil {
		return pm, fmt.Errorf("database ping failed: %w", err)
	}
	// Cache
	pm.Cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		return pm, err
	}
	// Router
	pm.Router = chi.NewRouter()
	pm.Router.Use(middleware.Recoverer)
	pm.Router.Get("/pm-admin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Welcome to the pagemanager dashboard")
	})
	var count int
	err = pm.DB.QueryRow("SELECT COUNT(*) FROM pm_routes").Scan(&count)
	if err != nil {
		log.Fatalln(err)
	}
	pm.Router.Get("/restart", func(w http.ResponseWriter, r *http.Request) {
		pm.Restart <- struct{}{}
		io.WriteString(w, "restarted\nThere are "+strconv.Itoa(count)+" rows in pm_routes")
	})
	return pm, nil
}

func (pm *PageManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// disabled?
	if value, ok := pm.Cache.Get(disabledKey + r.URL.Path); ok {
		if disabled, ok := value.(bool); ok {
			if disabled {
				pm.Router.NotFoundHandler().ServeHTTP(w, r)
				return
			}
		} else {
			pm.Cache.Del(disabledKey + r.URL.Path)
		}
	}
	// page cached?
	if value, ok := pm.Cache.Get(pageKey + r.URL.Path); ok {
		if page, ok := value.(string); ok {
			io.WriteString(w, page)
			return
		} else {
			pm.Cache.Del(pageKey + r.URL.Path)
		}
	}
	// redirect_url cached?
	if value, ok := pm.Cache.Get(redirectKey + r.URL.Path); ok {
		if redirectURL, ok := value.(string); ok {
			if redirectURL != "" {
				http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
				return
			}
		} else {
			pm.Cache.Del(redirectKey + r.URL.Path)
		}
	}
	// handler_url cached?
	if value, ok := pm.Cache.Get(handlerKey + r.URL.Path); ok {
		if handlerURL, ok := value.(string); ok {
			handler := lookupHandler(pm.Router, handlerURL)
			if handler != nil {
				handler.ServeHTTP(w, r)
				return
			} else {
				pm.Cache.Del(handlerKey + r.URL.Path)
			}
		} else {
			pm.Cache.Del(handlerKey + r.URL.Path)
		}
	}
	var disabled sql.NullBool
	var page, redirectURL, handlerURL sql.NullString
	err := pm.DB.
		QueryRow("SELECT disabled, page, redirect_url, handler_url FROM pm_routes WHERE url = ?", r.URL.Path).
		Scan(&disabled, &page, &redirectURL, &handlerURL)
	if errors.Is(err, sql.ErrNoRows) {
		pm.Router.ServeHTTP(w, r)
		return
	}
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	if disabled.Bool {
		pm.Cache.Set(disabledKey+r.URL.Path, disabled.Bool, 0)
		pm.Router.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	if page.String != "" {
		pm.Cache.Set(pageKey+r.URL.Path, page.String, 0)
		io.WriteString(w, page.String)
		return
	}
	if redirectURL.String != "" {
		pm.Cache.Set(handlerKey+r.URL.Path, redirectURL.String, 0)
		if redirectURL.String != "" {
			http.Redirect(w, r, redirectURL.String, http.StatusMovedPermanently)
			return
		}
	}
	if handlerURL.String != "" {
		handler := lookupHandler(pm.Router, handlerURL.String)
		if handler != nil {
			pm.Cache.Set(handlerKey+r.URL.Path, handlerURL.String, 0)
			handler.ServeHTTP(w, r)
			return
		}
	}
	pm.Router.ServeHTTP(w, r)
}

func lookupHandler(mux *chi.Mux, path string) http.Handler {
	rctx := chi.NewRouteContext()
	_, _, handler := mux.Tree.FindRoute(rctx, chi.MGET, path)
	return handler
}

func (pm *PageManager) SetPage(url, page string) error {
	_, err := pm.DB.Exec(
		"INSERT INTO pm_routes (url, page) VALUES (?, ?) ON CONFLICT (url) DO UPDATE SET page = EXCLUDED.page",
		url, page,
	)
	if err != nil {
		return err
	}
	pm.Cache.Set(pageKey+url, page, 0)
	return nil
}

func (pm *PageManager) DelPage(url string) error {
	_, err := pm.DB.Exec("UPDATE pm_routes SET page = NULL WHERE url = ?", url)
	if err != nil {
		return err
	}
	pm.Cache.Del(pageKey + url)
	return nil
}

type Plugin interface {
	AddRoutes() error
}

func (pm *PageManager) AddPlugins(constructors ...func(*PageManager) Plugin) error {
	var err error
	var plugin Plugin
	for _, constructor := range constructors {
		plugin = constructor(pm)
		err = plugin.AddRoutes()
		if err != nil {
			return err
		}
	}
	return nil
}
