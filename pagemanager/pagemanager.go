package pagemanager

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sq "github.com/bokwoon95/go-structured-query/postgres"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
	"github.com/dgraph-io/ristretto"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/pelletier/go-toml"
)

const (
	pageKey     = "pageKey\x00"
	redirectKey = "redirectKey\x00"
	handlerKey  = "handlerKey\x00"
	disabledKey = "disabledKey\x00"
)

type PageManager struct {
	Restart       chan struct{}
	DB            *sql.DB
	cache         *ristretto.Cache
	Router        *chi.Mux
	htmlPolicy    *bluemonday.Policy
	RootDirectory string
	Render        *renderly.Renderly
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
	pm.cache, err = ristretto.NewCache(&ristretto.Config{
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
	pm.Router.Use(pm.pm_routes)
	pm.Router.Use(SecurityHeaders)
	pm.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		chi.Walk(pm.Router, printroutes(w))
		// io.WriteString(w, docgen.JSONRoutesDoc(pm.Router))
	})
	pm.Router.Get("/pm-admin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Welcome to the pagemanager dashboard")
	})
	pm.Router.Post("/pm-kv", pm.KVPost)
	pm.Router.Post("/restart", func(w http.ResponseWriter, r *http.Request) {
		pm.Restart <- struct{}{}
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})
	// HTMLPolicy
	pm.htmlPolicy = bluemonday.UGCPolicy()
	pm.htmlPolicy.AllowStyling()
	// RootDirectory
	pm.RootDirectory = "." + string(os.PathSeparator) + "pagemanager" + string(os.PathSeparator)
	// renderly
	pm.Render, err = renderly.New(os.DirFS("./templates"))
	if err != nil {
		return pm, err
	}
	fsys, name := pm.Render.Resolve("plainsimple/post-index.html")
	t, dir, err := locateconfig(fsys, name)
	if err != nil {
		return pm, err
	}
	key := strings.TrimPrefix("plainsimple/post-index.html", dir+string(os.PathSeparator))
	// TODO: figure out how to get a subtree, I keep getting <nil> for some reason.
	m := t.ToMap()
	fmt.Println(m[key])
	return pm, nil
}

func (pm *PageManager) pm_routes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if value, ok := pm.cache.Get(disabledKey + r.URL.Path); ok {
			if disabled, ok := value.(bool); ok {
				if disabled {
					pm.Router.NotFoundHandler().ServeHTTP(w, r)
					return
				}
			} else {
				pm.cache.Del(disabledKey + r.URL.Path)
			}
		}
		// page cached?
		if value, ok := pm.cache.Get(pageKey + r.URL.Path); ok {
			if page, ok := value.(string); ok {
				fmt.Println("serving page", r.URL, "from cache")
				io.WriteString(w, page)
				return
			} else {
				pm.cache.Del(pageKey + r.URL.Path)
			}
		}
		// redirect_url cached?
		if value, ok := pm.cache.Get(redirectKey + r.URL.Path); ok {
			if redirectURL, ok := value.(string); ok {
				if redirectURL != "" {
					http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
					return
				}
			} else {
				pm.cache.Del(redirectKey + r.URL.Path)
			}
		}
		// handler_url cached?
		if value, ok := pm.cache.Get(handlerKey + r.URL.Path); ok {
			if handlerURL, ok := value.(string); ok {
				rctx := chi.RouteContext(r.Context())
				if pm.Router.Match(chi.NewRouteContext(), "GET", handlerURL) {
					rctx.RoutePath = handlerURL
					next.ServeHTTP(w, r)
					return
				} else {
					pm.cache.Del(handlerKey + r.URL.Path)
				}
			} else {
				pm.cache.Del(handlerKey + r.URL.Path)
			}
		}
		var disabled sql.NullBool
		var page, redirectURL, handlerURL sql.NullString
		err := pm.DB.
			QueryRow("SELECT disabled, page, redirect_url, handler_url FROM pm_routes WHERE url = ?", r.URL.Path).
			Scan(&disabled, &page, &redirectURL, &handlerURL)
		if errors.Is(err, sql.ErrNoRows) {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		if disabled.Bool {
			pm.cache.Set(disabledKey+r.URL.Path, disabled.Bool, 0)
			pm.Router.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		if page.String != "" {
			pm.cache.Set(pageKey+r.URL.Path, page.String, 0)
			io.WriteString(w, page.String)
			return
		}
		if redirectURL.String != "" {
			pm.cache.Set(handlerKey+r.URL.Path, redirectURL.String, 0)
			if redirectURL.String != "" {
				http.Redirect(w, r, redirectURL.String, http.StatusMovedPermanently)
				return
			}
		}
		if handlerURL.String != "" {
			rctx := chi.RouteContext(r.Context())
			if pm.Router.Match(chi.NewRouteContext(), "GET", handlerURL.String) {
				rctx.RoutePath = handlerURL.String
				next.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (pm *PageManager) pm_routesv2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, found := pm.cache.Get(r.URL.Path)
		route, ok := data.(Route)
		if !found || !ok {
			query := "SELECT url, disabled, redirect_url, handler_url, content, page FROM pm_routes WHERE url = ?"
			err := pm.DB.
				QueryRow(query, r.URL.Path).
				Scan(&route.URL, &route.Disabled, &route.RedirectURL, &route.HandlerURL, &route.Content, &route.Page)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = pm.cache.Set(r.URL.Path, route, 0)
		}
		if route.Disabled.Valid && route.Disabled.Bool {
			pm.Router.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		if route.RedirectURL.Valid {
			http.Redirect(w, r, route.RedirectURL.String, http.StatusMovedPermanently)
			return
		}
		if route.HandlerURL.Valid {
			rctx := chi.RouteContext(r.Context())
			rctx.RoutePath = route.HandlerURL.String
			next.ServeHTTP(w, r)
			return
		}
		if route.Content.Valid {
			io.WriteString(w, route.Content.String)
			return
		}
		if route.Page.Valid {
			fsys, filename := pm.Render.Resolve(route.Page.String)
			if fsys == nil {
				http.Error(w, "can't locate fsys of "+route.Page.String, http.StatusInternalServerError)
			}
			_ = filename
		}
		next.ServeHTTP(w, r)
	})
}

// Locate config.toml
func locateconfig(fsys fs.FS, filename string) (t *toml.Tree, dir string, err error) {
	var currentDir, newDir, tomlpath string
	currentDir = filename
UP_DIR:
	for {
		newDir = filepath.Dir(currentDir)
		if newDir == currentDir {
			break
		}
		currentDir = newDir
		f, err := fsys.Open(currentDir)
		if err != nil {
			return nil, currentDir, err
		}
		defer f.Close()
		fdir, ok := f.(interface {
			Readdirnames(n int) (names []string, err error)
		})
		if !ok {
			return nil, currentDir, fmt.Errorf("can't read dir")
		}
		for {
			names, err := fdir.Readdirnames(1)
			if err != nil {
				if errors.Is(err, io.EOF) {
					continue UP_DIR // no more files in current directory, go up one more directory
				}
				return nil, currentDir, err
			}
			if names[0] == "config.toml" {
				if currentDir == "." {
					tomlpath = "config.toml"
				} else {
					tomlpath = currentDir + string(os.PathSeparator) + "config.toml"
				}
				goto LOAD_TOML
			}
		}
	}
LOAD_TOML:
	if tomlpath == "" {
		return nil, currentDir, fmt.Errorf("no config.toml found")
	}
	b, err := fs.ReadFile(fsys, tomlpath)
	if err != nil {
		return nil, currentDir, err
	}
	t, err = toml.LoadBytes(b)
	if err != nil {
		return nil, currentDir, err
	}
	return t, currentDir, nil
}

type PageSource struct {
	Name     string
	HTML     []string
	CSS      []string
	JS       []string
	MD       []string
	MainHTML string
	Args     map[string]interface{}
}

type Plugin interface {
	AddRoutes() error
}

type PluginConstructor func(*PageManager) (Plugin, error)

func (pm *PageManager) AddPlugins(constructors ...PluginConstructor) error {
	var err error
	var plugin Plugin
	for _, constructor := range constructors {
		plugin, err = constructor(pm)
		if err != nil {
			return err
		}
		err = plugin.AddRoutes()
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO: howtf to make this a template function accessible to everyone?
func (pm *PageManager) KVGet(key, value string) error {
	return nil
}

type KeyValuePostData struct {
	KeyValuePairs []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"key_value_pairs"`
	RedirectTo string `json:"redirect_to"`
}

func (pm *PageManager) KVPost(w http.ResponseWriter, r *http.Request) {
	kvdata := KeyValuePostData{}
	err := decodeJSONBody(w, r, &kvdata)
	if err != nil {
		var mr *malformedRequest
		switch {
		case errors.As(err, &mr):
			http.Error(w, mr.msg, mr.status)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	pm_kv := pm_kv()
	_, err = sq.
		InsertInto(pm_kv).
		Valuesx(func(col *sq.Column) {
			for _, keyValuePair := range kvdata.KeyValuePairs {
				col.SetString(pm_kv.key, keyValuePair.Key)
				col.SetString(pm_kv.value, keyValuePair.Value)
			}
		}).
		OnConflict(pm_kv.key).
		DoUpdateSet(pm_kv.value.Set(sq.Excluded(pm_kv.value))).
		Exec(pm.DB, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	for _, keyValuePair := range kvdata.KeyValuePairs {
		pm.cache.Set(keyValuePair.Key, keyValuePair.Value, 0)
	}
	http.Redirect(w, r, kvdata.RedirectTo, http.StatusMovedPermanently)
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityPolicies := []string{
			`script-src-elem
				'self'
				cdn.jsdelivr.net
				stackpath.bootstrapcdn.com
				cdn.datatables.net
				unpkg.com
				code.jquery.com
			`,
			`style-src-elem
				'self'
				cdn.jsdelivr.net
				stackpath.bootstrapcdn.com
				cdn.datatables.net
				unpkg.com
				fonts.googleapis.com
			`,
			`img-src
				'self'
				cdn.datatables.net
				data:
				source.unsplash.com
				images.unsplash.com
			`,
			`font-src fonts.gstatic.com`,
			"default-src 'self'",
			"object-src 'self'",
			"media-src 'self'",
			"frame-ancestors 'self'",
			"connect-src 'self'",
		}
		ContentSecurityPolicy := regexp.MustCompile(`\s+`).ReplaceAllString(strings.Join(securityPolicies, "; "), " ")
		w.Header().Set("Content-Security-Policy", ContentSecurityPolicy)
		features := []string{
			`microphone 'none'`,
			`camera 'none'`,
			`magnetometer 'none'`,
			`gyroscope 'none'`,
		}
		FeaturePolicy := regexp.MustCompile(`\s+`).ReplaceAllString(strings.Join(features, "; "), " ")
		w.Header().Set("Feature-Policy", FeaturePolicy)
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "sameorigin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}
