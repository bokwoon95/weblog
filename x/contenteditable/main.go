package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/bokwoon95/weblog/pagemanager"
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
