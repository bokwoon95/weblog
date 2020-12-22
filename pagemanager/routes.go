package pagemanager

import (
	"fmt"
	"io"
	"net/http"
)

func printroutes(w io.Writer) func(string, string, http.Handler, ...func(http.Handler) http.Handler) error {
	return func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if method == "GET" {
			fmt.Fprintln(w, route, "[internal handler]")
		}
		return nil
	}
}

type Route struct {
	URL         string
	Disabled    bool
	RedirectURL string
	HandlerURL  string
	Content     string
}
