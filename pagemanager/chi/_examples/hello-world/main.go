package main

import (
	"net/http"

	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/pagemanager/chi/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})

	http.ListenAndServe(":3333", r)
}
