package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/bokwoon95/weblog/blog"
	"github.com/bokwoon95/weblog/pagemanager"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
)

const port = ":80"

func main() {
	fmt.Println(os.Executable())
	for {
		pm, err := pagemanager.New("sqlite3", "./database.sqlite3")
		if err != nil {
			log.Fatalln(err)
		}
		err = pm.AddPlugins(blog.New("blog"))
		if err != nil {
			log.Fatalln(err)
		}
		tmp := os.DirFS(renderly.AbsDir("./blog"))
		templatesDir := os.DirFS("./templates/plainsimple")
		render, err := renderly.New(
			os.DirFS(renderly.AbsDir(".")),
			renderly.AltFS("blog", tmp),
			renderly.AltFS("templates", templatesDir),
		)
		if err != nil {
			log.Fatalln(err)
		}
		pm.Router.Handle("/static/*", http.StripPrefix("/static/", render.FileServer()))
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
