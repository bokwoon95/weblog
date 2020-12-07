package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bokwoon95/weblog/blog"
	"github.com/bokwoon95/weblog/pagemanager"
)

const port = ":80"

func main() {
	server, err := pagemanager.NewServer("sqlite3", "./weblog.sqlite3")
	if err != nil {
		log.Fatalln(err)
	}
	err = server.AddPlugins(blog.New("blog"))
	if err != nil {
		log.Fatalln(err)
	}
	server.Router.Get("/pm-admin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Welcome to the pagemanager dashboard")
	})
	defer func() { // only works for sqlite3
		_, _ = server.DB.Exec("PRAGMA optimize")
	}()
	fmt.Println("Listening on localhost" + port)
	log.Fatal(http.ListenAndServe(port, server))
}
