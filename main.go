package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bokwoon95/weblog/pagemanager"
)

const port = ":80"

func main() {
	server, err := pagemanager.NewServer("sqlite3", "./weblog.sqlite3")
	if err != nil {
		log.Fatalln(err)
	}
	server.Router.Get("/pm-admin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Welcome to the pagemanager dashboard")
	})
	fmt.Println("Listening on localhost" + port)
	http.ListenAndServe(port, server)
}
