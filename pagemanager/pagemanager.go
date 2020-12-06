package pagemanager

import (
	"database/sql"
	"errors"
	"io"
	"net/http"

	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/pagemanager/chi/middleware"
	"github.com/dgraph-io/ristretto"
	_ "github.com/mattn/go-sqlite3"
)

const (
	pageKey     = "pageKey" + string(rune(0))
	handlerKey  = "handlerKey" + string(rune(0))
	disabledKey = "disabledKey" + string(rune(0))
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
	if value, ok := srv.Cache.Get(pageKey + r.URL.Path); ok {
		if page, ok := value.(string); ok {
			io.WriteString(w, page)
			return
		} else {
			srv.Cache.Del(pageKey + r.URL.Path)
		}
	}
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
	query := "SELECT disabled, page, handler_url FROM pm_routes WHERE url = ?"
	var disabled sql.NullBool
	var page, handlerURL sql.NullString
	err := srv.DB.QueryRow(query, r.URL.Path).Scan(&disabled, &page, &handlerURL)
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
