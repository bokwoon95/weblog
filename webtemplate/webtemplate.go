package webtemplate

import (
	"html/template"
	"net/http"

	"github.com/oxtoacart/bpool"
)

type Source struct {
	// equivalent html/template call:
	// t.New(src.Name).Funcs(src.Funcs).Option(src.Options...).Parse(src.Text)
	Name      string
	Filepaths []string
	Text      string
	Funcs     map[string]interface{}
	Options   []string
}

type Sources struct {
	Templates       []Source
	GlobalTemplates []Source
	GlobalFuncs     map[string]interface{}
	GlobalOptions   []string
}

type Templates struct {
	bufpool *bpool.BufferPool
	global  *template.Template            // gets included in every template in the cache
	lib     map[string]*template.Template // never gets executed, main purpose for cloning
	cache   map[string]*template.Template // is what gets executed, should not changed after it is set
	funcs   map[string]interface{}
	options []string
}

type ParseOption func(*Sources) error

func addParseTree(parent *template.Template, child *template.Template) error {
	var err error
	for _, t := range child.Templates() {
		_, err = parent.AddParseTree(t.Name(), t.Tree)
		if err != nil {
			return err
		}
	}
	return nil
}

func Parse(opts ...ParseOption) (*Templates, error) {
	ts := &Templates{
		bufpool: bpool.NewBufferPool(64),
		global:  template.New(""),
		lib:     make(map[string]*template.Template),
		cache:   make(map[string]*template.Template),
	}
	srcs := &Sources{
		GlobalFuncs: make(map[string]interface{}),
	}
	var err error
	for _, opt := range opts {
		err = opt(srcs)
		if err != nil {
			return ts, err
		}
	}
	if len(srcs.GlobalFuncs) > 0 {
		ts.global = ts.global.Funcs(srcs.GlobalFuncs)
	}
	if len(srcs.GlobalOptions) > 0 {
		ts.global = ts.global.Option(srcs.GlobalOptions...)
	}
	for _, src := range srcs.GlobalTemplates {
		ts.global, err = ts.global.New(src.Name).Parse(src.Text)
		if err != nil {
			return ts, err
		}
	}
	for _, src := range srcs.Templates {
		var tmpl, cacheEntry *template.Template
		tmpl, err = template.New(src.Name).Funcs(srcs.GlobalFuncs).Option(srcs.GlobalOptions...).Parse(src.Text)
		if err != nil {
			return ts, err
		}
		ts.lib[src.Name] = tmpl
		cacheEntry, err = ts.global.Clone()
		if err != nil {
			return ts, err
		}
		err = addParseTree(cacheEntry, tmpl)
		if err != nil {
			return ts, err
		}
		ts.cache[src.Name] = cacheEntry
	}
	return ts, nil
}

func (tpt *Templates) Parse(opts ...ParseOption) error {
	srcs := &Sources{
		GlobalFuncs: make(map[string]interface{}),
	}
	var err error
	for _, opt := range opts {
		err = opt(srcs)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddFiles(filenames ...string) func(*Sources) {
	return func(srcs *Sources) {
	}
}

func (tpt *Templates) Clone() (*Templates, error) {
	newTpt := &Templates{}
	return newTpt, nil
}

func (tpt *Templates) Render(w http.ResponseWriter, r *http.Request, data map[string]interface{}, name string, names ...string) error {
	// if name cannot be found in ts.lib, make sure to check ts.common.Lookup() first. The user may be trying to render a global template
	return nil
}
