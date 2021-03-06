package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bokwoon95/weblog/blog"
	"github.com/bokwoon95/weblog/pagemanager"
)

const port = ":80"

func main() {
	a, err := os.Executable()
	if err != nil {
		log.Fatalln(err)
	}
	b, err := filepath.EvalSymlinks(a)
	if err != nil {
		log.Fatalln()
	}
	fmt.Println(b)
	for {
		pm, err := pagemanager.New("sqlite3", "./database.sqlite3")
		if err != nil {
			log.Fatalln(err)
		}
		err = pm.AddPlugins(blog.New("blog"))
		if err != nil {
			log.Fatalln(err)
		}
		defer func() { // only works for sqlite3
			_, _ = pm.DB.Exec("PRAGMA optimize")
		}()
		srv := http.Server{
			Addr:    port,
			Handler: pm.Router,
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
