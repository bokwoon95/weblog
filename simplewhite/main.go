package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/pagemanager/templat"
)

var (
	_, sourcefile, _, _ = runtime.Caller(0)
	sourcedir           = filepath.Dir(sourcefile)
)

var templates *templat.Templates

const lipsum = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Quisque auctor aliquam elit a iaculis. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas enim diam, scelerisque sed magna vitae, iaculis ornare eros. Suspendisse placerat mollis dolor. Donec non convallis justo. Cras et enim neque. Fusce mattis lacinia turpis vitae sollicitudin. Praesent a leo quis dui aliquam blandit sit amet ut felis."

type Post struct {
	Title   string
	Date    time.Time
	Summary string
	Body    string
}

var posts = []Post{
	{
		Title:   "Title One",
		Date:    time.Now(),
		Summary: "the quick brown fox jumped over the lazy dog",
		Body:    lipsum,
	},
	{
		Title:   "Title Two",
		Date:    time.Now(),
		Summary: "the quick brown fox jumped over the lazy dog",
		Body:    lipsum,
	},
	{
		Title:   "Title Three",
		Date:    time.Now(),
		Summary: "the quick brown fox jumped over the lazy dog",
		Body:    lipsum,
	},
	{
		Title:   "Title Four",
		Date:    time.Now(),
		Summary: "the quick brown fox jumped over the lazy dog",
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
	var err error
	common := []string{
		sourcedir + "/header.html",
	}
	if false {
		common = append(common, sourcedir+"/index_post_summary.html")
		common = append(common, sourcedir+"/pagination_none.html")
	} else {
		common = append(common, sourcedir+"/index_post_body.html")
		common = append(common, sourcedir+"/pagination_yes.html")
	}
	templates, err = templat.Parse(common, []string{
		sourcedir + "/index.html",
		sourcedir + "/post.html",
	}, templat.Funcs(funcs))
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(templates.DefinedTemplates())
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"posts":  posts,
		}
		err := templates.Render(w, r, data, sourcedir+"/index.html")
		if err != nil {
			log.Fatalln(err)
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
