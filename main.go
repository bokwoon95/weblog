package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bokwoon95/weblog/blog"
	"github.com/bokwoon95/weblog/pagemanager"
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
		var count int
		err = pm.DB.QueryRow("SELECT COUNT(*) FROM pm_routes").Scan(&count)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println("count:", count)
		pm.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "restarted\nThere are "+strconv.Itoa(count)+" rows in pm_routes")
		})
		fmt.Println("Listening on localhost" + srv.Addr)
		// go func() {
		// 	time.Sleep(100 * time.Millisecond)
		// 	switch runtime.GOOS {
		// 	case "windows":
		// 		exec.Command("start", "http://localhost:80").Start()
		// 	case "darwin":
		// 		exec.Command("open", "http://localhost:80").Start()
		// 	case "linux":
		// 		exec.Command("xdg-open", "http://localhost:80").Start()
		// 	}
		// }()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("srv.ListenAndServe error: %v\n", err)
		}
	}
}
