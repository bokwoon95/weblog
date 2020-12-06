package pagemanager

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

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

type Server struct {
	DB     *sql.DB
	Cache  *ristretto.Cache
	Router *chi.Mux
}

func NewServer(driverName, dataSourceName string) (*Server, error) {
	var err error
	srv := &Server{}
	// DB
	srv.DB, err = sql.Open(driverName, dataSourceName)
	if err != nil {
		return srv, err
	}
	if driverName == "sqlite3" {
		_, err = srv.DB.Exec("PRAGMA journal_mode = WAL")
		if err != nil {
			return srv, err
		}
		_, err = srv.DB.Exec("PRAGMA synchronous = normal")
		if err != nil {
			return srv, err
		}
		_, err = srv.DB.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			return srv, err
		}
	}
	err = srv.DB.Ping()
	if err != nil {
		return srv, fmt.Errorf("database ping failed: %w", err)
	}
	// Cache
	srv.Cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		return srv, err
	}
	// Router
	srv.Router = chi.NewRouter()
	srv.Router.Use(middleware.Recoverer)
	return srv, nil
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// disabled?
	if value, ok := srv.Cache.Get(disabledKey + r.URL.Path); ok {
		if disabled, ok := value.(bool); ok {
			if disabled {
				srv.Router.NotFoundHandler().ServeHTTP(w, r)
				return
			}
		} else {
			srv.Cache.Del(disabledKey + r.URL.Path)
		}
	}
	// page cached?
	if value, ok := srv.Cache.Get(pageKey + r.URL.Path); ok {
		if page, ok := value.(string); ok {
			io.WriteString(w, page)
			return
		} else {
			srv.Cache.Del(pageKey + r.URL.Path)
		}
	}
	// redirect_url cached?
	if value, ok := srv.Cache.Get(redirectKey + r.URL.Path); ok {
		if redirectURL, ok := value.(string); ok {
			if redirectURL != "" {
				http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
				return
			}
		} else {
			srv.Cache.Del(redirectKey + r.URL.Path)
		}
	}
	// handler_url cached?
	if value, ok := srv.Cache.Get(handlerKey + r.URL.Path); ok {
		if handlerURL, ok := value.(string); ok {
			handler := lookupHandler(srv.Router, handlerURL)
			if handler != nil {
				handler.ServeHTTP(w, r)
				return
			} else {
				srv.Cache.Del(handlerKey + r.URL.Path)
			}
		} else {
			srv.Cache.Del(handlerKey + r.URL.Path)
		}
	}
	var disabled sql.NullBool
	var page, redirectURL, handlerURL sql.NullString
	err := srv.DB.
		QueryRow("SELECT disabled, page, redirect_url, handler_url FROM pm_routes WHERE url = ?", r.URL.Path).
		Scan(&disabled, &page, &redirectURL, &handlerURL)
	if errors.Is(err, sql.ErrNoRows) {
		srv.Router.ServeHTTP(w, r)
		return
	}
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	if disabled.Bool {
		srv.Cache.Set(disabledKey+r.URL.Path, disabled.Bool, 0)
		srv.Router.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	if page.String != "" {
		srv.Cache.Set(pageKey+r.URL.Path, page.String, 0)
		io.WriteString(w, page.String)
		return
	}
	if redirectURL.String != "" {
		srv.Cache.Set(handlerKey+r.URL.Path, redirectURL.String, 0)
		if redirectURL.String != "" {
			http.Redirect(w, r, redirectURL.String, http.StatusMovedPermanently)
			return
		}
	}
	if handlerURL.String != "" {
		handler := lookupHandler(srv.Router, handlerURL.String)
		if handler != nil {
			srv.Cache.Set(handlerKey+r.URL.Path, handlerURL.String, 0)
			handler.ServeHTTP(w, r)
			return
		}
	}
	srv.Router.ServeHTTP(w, r)
}

func lookupHandler(mux *chi.Mux, path string) http.Handler {
	rctx := chi.NewRouteContext()
	_, _, handler := mux.Tree.FindRoute(rctx, chi.MGET, path)
	return handler
}

func (srv *Server) SetPage(url, page string) error {
	_, err := srv.DB.Exec(
		"INSERT INTO pm_routes (url, page) VALUES (?, ?) ON CONFLICT (url) DO UPDATE SET page = EXCLUDED.page",
		url, page,
	)
	if err != nil {
		return err
	}
	srv.Cache.Set(pageKey+url, page, 0)
	return nil
}

func (srv *Server) DelPage(url string) error {
	_, err := srv.DB.Exec("UPDATE pm_routes SET page = NULL WHERE url = ?", url)
	if err != nil {
		return err
	}
	srv.Cache.Del(pageKey + url)
	return nil
}

type Plugin interface {
	AddRoutes() error
}

func (srv *Server) AddPlugins(constructors ...func(*Server) Plugin) error {
	var err error
	var plugin Plugin
	for _, constructor := range constructors {
		plugin = constructor(srv)
		err = plugin.AddRoutes()
		if err != nil {
			return err
		}
	}
	return nil
}
