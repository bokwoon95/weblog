package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/bokwoon95/weblog/pagemanager/renderly"
)

var (
	_, sourcefile, _, _ = runtime.Caller(0)
	sourcedir           = filepath.Dir(sourcefile)
)

const lipsum = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Quisque auctor aliquam elit a iaculis. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas enim diam, scelerisque sed magna vitae, iaculis ornare eros. Suspendisse placerat mollis dolor. Donec non convallis justo. Cras et enim neque. Fusce mattis lacinia turpis vitae sollicitudin. Praesent a leo quis dui aliquam blandit sit amet ut felis."

type Post struct {
	Title   string
	Date    time.Time
	Summary string
	Body    string
}

var singapore, _ = time.LoadLocation("Asia/Singapore")

var posts = []Post{
	{
		Title:   "HASH: a free, online platform for modeling the world",
		Date:    time.Date(2020, 6, 18, 0, 0, 0, 0, singapore),
		Summary: "Sometimes simulating complex systems is the best way to understand them.",
		Body:    lipsum,
	},
	{
		Title:   "So, how’s that retirement thing going, anyway?",
		Date:    time.Date(2019, 12, 5, 0, 0, 0, 0, singapore),
		Summary: "For the last couple of months, Prashanth Chandrasekar has been getting settled in as the new CEO of Stack Overflow. I’m still going on some customer calls…",
		Body:    lipsum,
	},
	{
		Title:   "Welcome, Prashanth!",
		Date:    time.Date(2019, 9, 24, 0, 0, 0, 0, singapore),
		Summary: "Last March, I shared that we were starting to look for a new CEO for Stack Overflow. We were looking for that rare combination of someone who…",
		Body:    lipsum,
	},
	{
		Title:   "The next CEO of Stack Overflow",
		Date:    time.Date(2019, 3, 28, 0, 0, 0, 0, singapore),
		Summary: "We’re looking for a new CEO for Stack Overflow. I’m stepping out of the day-to-day and up to the role of Chairman of the Board.",
		Body:    lipsum,
	},
}

var funcs = map[string]interface{}{
	"mapadd": func(base map[string]interface{}, args ...interface{}) map[string]interface{} {
		newbase := make(map[string]interface{})
		for key, value := range base {
			newbase[key] = value
		}
		for i := 0; i < len(args); i += 2 {
			if i+1 >= len(args) {
				break
			}
			key := fmt.Sprint(args[i])
			value := args[i+1]
			newbase[key] = value
		}
		return newbase
	},
}

func main() {
	fsys := os.DirFS(renderly.AbsDir("."))
	var opts []renderly.Option
	if true {
		opts = append(opts, renderly.GlobalTemplates(fsys, "index_post_summary.html", "pagination_none.html"))
	} else {
		opts = append(opts, renderly.GlobalTemplates(fsys, "index_post_body.html", "pagination_yes.html"))
	}
	render, err := renderly.New(fsys, opts...)
	if err != nil {
		log.Fatalln(err)
	}
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"posts": posts,
		}
		err := render.Page(w, r, data, "index.html")
		if err != nil {
			io.WriteString(w, err.Error())
		}
	})
	FileServer(r, "/static", http.Dir(sourcedir))
	fmt.Println("Listening on localhost:8080")
	http.ListenAndServe(":8080", r)
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
