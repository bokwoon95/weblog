package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
)

var (
	_, sourcefile, _, _ = runtime.Caller(0)
	sourcedir           = filepath.Dir(sourcefile)
)

const port = ":80"

func main() {
	for {
		pm, err := pagemanager.New("sqlite3", "./weblog.sqlite3")
		if err != nil {
			log.Fatalln(err)
		}
		err = pm.AddPlugins(New)
		if err != nil {
			log.Fatalln(err)
		}
		defer func() { // only works for sqlite3
			_, _ = pm.DB.Exec("PRAGMA optimize")
		}()
		srv := http.Server{
			Addr:    port,
			Handler: pm,
		}
		go func() {
			<-pm.Restart
			if err := srv.Shutdown(context.Background()); err != nil {
				log.Printf("srv.Shutdown error: %v\n", err)
			}
		}()
		fmt.Println("Listening on localhost" + srv.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("srv.ListenAndServe error: %v\n", err)
		}
	}
}

type Server struct {
	*pagemanager.PageManager
	Render *renderly.Renderly
}

func New(pm *pagemanager.PageManager) (pagemanager.Plugin, error) {
	plugin := &Server{
		PageManager: pm,
	}
	var err error
	fsys := os.DirFS(renderly.AbsDir("."))
	plugin.Render, err = renderly.New(fsys)
	if err != nil {
		return plugin, err
	}
	return plugin, nil
}

func (srv *Server) AddRoutes() error {
	srv.Router.Get("/", srv.User)
	srv.Router.Post("/preview", srv.User)
	return nil
}

func (srv *Server) User(w http.ResponseWriter, r *http.Request) {
	srv.Render.Page(w, r, nil, "tachyons.css", "main.html", "main.css", "main.js")
}
