package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/bokwoon95/weblog/pagemanager/chi"
)

const start = `script-src-elem 'self' 'unsafe-inline' 'nonce-iKhbjLq7WUIq9DKSONPbAa6E_bxuIWskJYBY39RE6UU=' cdn.jsdelivr.net stackpath.bootstrapcdn.com cdn.datatables.net unpkg.com code.jquery.com ;` +
	` style-src-elem 'self' cdn.jsdelivr.net stackpath.bootstrapcdn.com cdn.datatables.net unpkg.com fonts.googleapis.com ;` +
	` style-src 'unsafe-inline';` +
	` img-src 'self' 'unsafe-inline' cdn.datatables.net data: source.unsplash.com images.unsplash.com ;` +
	` font-src fonts.gstatic.com;` +
	` default-src 'self';` +
	` object-src 'self';` +
	` media-src 'self';` +
	` frame-ancestors 'self';` +
	` connect-src 'self'`

const middle = `font-src fonts.gstatic.com;` +
	` default-src 'self';` +
	` object-src 'self';` +
	` media-src 'self';` +
	` frame-ancestors 'self';` +
	` script-src-elem 'self' 'unsafe-inline' 'nonce-iKhbjLq7WUIq9DKSONPbAa6E_bxuIWskJYBY39RE6UU=' cdn.jsdelivr.net stackpath.bootstrapcdn.com cdn.datatables.net unpkg.com code.jquery.com ;` +
	` style-src-elem 'self' cdn.jsdelivr.net stackpath.bootstrapcdn.com cdn.datatables.net unpkg.com fonts.googleapis.com ;` +
	` style-src 'unsafe-inline';` +
	` img-src 'self' 'unsafe-inline' cdn.datatables.net data: source.unsplash.com images.unsplash.com ;` +
	` connect-src 'self'`

const end = `script-src-elem 'self' 'unsafe-inline' 'nonce-iKhbjLq7WUIq9DKSONPbAa6E_bxuIWskJYBY39RE6UU=' cdn.jsdelivr.net stackpath.bootstrapcdn.com cdn.datatables.net unpkg.com code.jquery.com`

const nothing = `lorem ipsum; ayayay`

const key = "Content-Security-Policy"

func main() {
	r := chi.NewRouter()
	r.Use(mw1, mw2)
	r.Get("/", handler)
	fmt.Println("listening on :8080")
	http.ListenAndServe(":8080", r)
}

func mw1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(key, nothing)
		next.ServeHTTP(w, r)
	})
}

func mw2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appendCSP(w, "script-src-elem", "'OOGA-BOOGA'")
		appendCSP(w, "script-src-elem", "'schumga-loola'")
		next.ServeHTTP(w, r)
	})
}

func appendCSP(w http.ResponseWriter, policy, value string) error {
	CSP := w.Header().Get(key)
	if CSP == "" {
		w.Header().Set(key, policy+" "+value)
		return nil
	}
	CSP = strings.ReplaceAll(CSP, "\n", " ") // newlines screw up the regex matching, remove them
	re, err := regexp.Compile(`(.*` + policy + `[^;]*)(;|$)(.*)`)
	if err != nil {
		return err
	}
	matches := re.FindStringSubmatch(CSP)
	if len(matches) == 0 {
		w.Header().Set(key, CSP+"; "+policy+" "+value)
		return nil
	}
	newCSP := matches[1] + " " + value + matches[2] + matches[3]
	w.Header().Set("Content-Security-Policy", newCSP)
	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	value := w.Header().Get(key)
	w.Write([]byte(value))
}
