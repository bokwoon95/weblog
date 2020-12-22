package pagemanager

import (
	"database/sql"
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
	URL         sql.NullString
	Disabled    sql.NullBool
	RedirectURL sql.NullString
	HandlerURL  sql.NullString
	Content     sql.NullString
	Page        sql.NullString
}
