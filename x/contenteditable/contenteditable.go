package main

import (
	"net/http"

	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/templat"
)

type Server struct {
	*pagemanager.PageManager
	templates *templat.Templates
}

func New(pm *pagemanager.PageManager) (pagemanager.Plugin, error) {
	plugin := &Server{
		PageManager: pm,
	}
	var err error
	x := []string{}
	y := []string{sourcedir + "/reader.html", sourcedir + "/user.html"}
	plugin.templates, err = templat.Parse(x, y)
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
	srv.templates.Render(w, r, nil, sourcedir+"/user.html")
}
