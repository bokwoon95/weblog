package wt

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/oxtoacart/bpool"
)

type Asset struct {
	File     Fyle
	Hash     string
	External bool
}

type Template struct {
	parent     *Templates
	Name       string
	NameInFile bool // how about I just don't bloody declare the template filename in the template? Then it would never filename conflict
	// You can do whatever you want in the associated templates. Just don't put a wrapper template over the main template.
	// yeah that works
	Funcs      map[string]interface{}
	Options    []string
	File       Fyle
	Associated []Template

	ExecuteName   string // ProxyName?
	HTML          *template.Template
	CSS           []Asset
	JS            []Asset
	Preprocessors []func(w http.ResponseWriter, r *http.Request, data map[string]interface{})

	// Common
}

type TemplatesSource struct {
	Templates []TemplatesSource
	Common    []TemplatesSource
}

type Templates struct {
	bufpool *bpool.BufferPool
	set     map[string]Template
	common  []Template
	mu      sync.RWMutex
}

type Option func(*Templates)

func New(opts ...Option) (*Templates, error) {
	return nil, nil
}

func (ts *Templates) Lookup(name string) Template {
	return Template{}
}

func (ts *Template) Render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
}

func (t *Template) ExecuteTemplate() {
}
