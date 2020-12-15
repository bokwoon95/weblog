package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bokwoon95/weblog/pagemanager/chi"
	"github.com/bokwoon95/weblog/webtemplate"
)

// original idea: all templat plugins provide a Funcs(in)out method, templat will group them together at parse time

// natural extension: how to share templates and funcmaps across pagemanager plugins?
// - templat plugins share templates and funcmaps by being defined together. this is where the css and js get defined.
// - pagemanager plugins will benefit from being able to access pagemanager's common set of funcs and templates.
//		so the real question is how to extend a templat templates. The problem is that it will def require an entire reparse because functions annoyingly can only be defined at parse time. You can't just addParseTree to the problem

const all_html = `
{{ range $i, $css := $.__wt_css__ }}
<style>
{{ $css.Text }}
</style>
{{ end }}
{{ template "side1" $ }}
{{ template "side2" $ }}
calling side2: {{ side2 }}
{{ template "main.html" $ }}
this is all
<br>
{{ range $i, $js := $.__wt_js__ }}
<script>
{{ $js.Text }}
</script>
{{ end }}
`

func main() {
	r := chi.NewRouter()
	wt, err := webtemplate.Parse(
		webtemplate.AddCommonFiles(webtemplate.Directory(0), "main.html"),
		webtemplate.AddSources(webtemplate.Source{
			Name: "all_html",
			Text: all_html,
		}),
		Side1,
		Side2,
	)
	if err != nil {
		log.Fatalln(err)
	}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		err := wt.Render(w, r, nil, "all_html")
		if err != nil {
			log.Println(err)
		}
	})
	fmt.Println("listening on :8080")
	http.ListenAndServe(":8080", r)
}
