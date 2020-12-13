package webtemplate

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

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
	CommonTemplates []Source
	CommonFuncs     map[string]interface{}
	CommonOptions   []string
}

type Template struct {
	Name string
	HTML *template.Template
	CSS  []template.CSS
	JS   []template.JS
}

type Templates struct {
	bufpool *bpool.BufferPool
	common  *template.Template            // gets included in every template in the cache
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
	var err error
	ts := &Templates{
		bufpool: bpool.NewBufferPool(64),
		common:  template.New(""),
		lib:     make(map[string]*template.Template),
		cache:   make(map[string]*template.Template),
	}
	srcs := &Sources{
		CommonFuncs: make(map[string]interface{}),
	}
	for _, opt := range opts {
		err = opt(srcs)
		if err != nil {
			return ts, err
		}
	}
	if len(srcs.CommonFuncs) > 0 {
		ts.common = ts.common.Funcs(srcs.CommonFuncs)
	}
	if len(srcs.CommonOptions) > 0 {
		ts.common = ts.common.Option(srcs.CommonOptions...)
	}
	for _, src := range srcs.CommonTemplates {
		ts.common, err = ts.common.New(src.Name).Parse(src.Text)
		if err != nil {
			return ts, err
		}
	}
	for _, src := range srcs.Templates {
		var tmpl, cacheEntry *template.Template
		tmpl, err = template.New(src.Name).Funcs(srcs.CommonFuncs).Option(srcs.CommonOptions...).Parse(src.Text)
		if err != nil {
			return ts, err
		}
		ts.lib[src.Name] = tmpl
		cacheEntry, err = ts.common.Clone()
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

func AddParse(base *Templates, opts ...ParseOption) (*Templates, error) {
	var err error
	ts := &Templates{
		bufpool: bpool.NewBufferPool(64),
		lib:     make(map[string]*template.Template),
		cache:   make(map[string]*template.Template),
	}
	// Clone base.common
	ts.common, err = base.common.Clone()
	if err != nil {
		return ts, err
	}
	// Clone base.lib and regenerate base.cache
	for name, tmpl := range base.lib {
		clonedTmpl, err := tmpl.Clone()
		if err != nil {
			return ts, err
		}
		ts.lib[name] = clonedTmpl
		cacheEntry, err := ts.common.Clone()
		if err != nil {
			return ts, err
		}
		err = addParseTree(cacheEntry, tmpl)
		if err != nil {
			return ts, err
		}
		ts.cache[name] = cacheEntry
	}
	srcs := &Sources{
		CommonFuncs: make(map[string]interface{}),
	}
	for _, opt := range opts {
		err = opt(srcs)
		if err != nil {
			return ts, err
		}
	}
	if len(srcs.CommonFuncs) > 0 {
		ts.common = ts.common.Funcs(srcs.CommonFuncs)
	}
	if len(srcs.CommonOptions) > 0 {
		ts.common = ts.common.Option(srcs.CommonOptions...)
	}
	for _, src := range srcs.CommonTemplates {
		ts.common, err = ts.common.New(src.Name).Parse(src.Text)
		if err != nil {
			return ts, err
		}
	}
	for _, src := range srcs.Templates {
		var tmpl, cacheEntry *template.Template
		tmpl, err = template.New(src.Name).Funcs(srcs.CommonFuncs).Option(srcs.CommonOptions...).Parse(src.Text)
		if err != nil {
			return ts, err
		}
		ts.lib[src.Name] = tmpl
		cacheEntry, err = ts.common.Clone()
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

func AddFiles(filepatterns ...string) ParseOption {
	return func(srcs *Sources) error {
		for _, filepattern := range filepatterns {
			filenames, err := filepath.Glob(filepattern)
			if err != nil {
				return err
			}
			for _, filename := range filenames {
				src := Source{}
				b, err := ioutil.ReadFile(filename)
				if err != nil {
					return err
				}
				src.Text = string(b)
				src.Filepaths = append(src.Filepaths, filename)
				// check if user already defined a template called `filename` inside the template itself
				re, err := regexp.Compile(`{{\s*define\s+["` + "`" + `]` + filename + `["` + "`" + `]\s*}}`)
				if err != nil {
					return err
				}
				if !re.MatchString(string(b)) {
					src.Name = filename
				}
				srcs.Templates = append(srcs.Templates, src)
			}
		}
		return nil
	}
}

func Funcs(funcs map[string]interface{}) ParseOption {
	return func(srcs *Sources) error {
		for name, fn := range funcs {
			srcs.CommonFuncs[name] = fn
		}
		return nil
	}
}

func Option(opts ...string) ParseOption {
	return func(srcs *Sources) error {
		srcs.CommonOptions = append(srcs.CommonOptions, opts...)
		return nil
	}
}

func lookup(ts *Templates, name string) (tmpl *template.Template, isCommon bool) {
	tmpl = ts.lib[name]
	if tmpl != nil {
		return tmpl, false
	}
	tmpl = ts.common.Lookup(name)
	if tmpl != nil {
		return tmpl, true
	}
	return nil, false
}

func executeTemplate(t *template.Template, w io.Writer, bufpool *bpool.BufferPool, name string, data map[string]interface{}) error {
	tempbuf := bufpool.Get()
	defer bufpool.Put(tempbuf)
	err := t.ExecuteTemplate(tempbuf, name, data)
	if err != nil {
		return err
	}
	_, err = tempbuf.WriteTo(w)
	if err != nil {
		return err
	}
	return nil
}

func (ts *Templates) Render(w http.ResponseWriter, r *http.Request, data map[string]interface{}, name string, names ...string) error {
	// TODO: check if render JSON
	// check if the template being rendered exists
	tmpl, isCommon := lookup(ts, name)
	if tmpl == nil {
		return fmt.Errorf("No such template '%s'\n", name)
	}
	if isCommon {
		err := executeTemplate(ts.common, w, ts.bufpool, name, data)
		if err != nil {
			return err
		}
		return nil
	}
	// used cached version if exists...
	fullname := strings.Join(append([]string{name}, names...), "\n")
	if tmpl, ok := ts.cache[fullname]; ok {
		err := executeTemplate(tmpl, w, ts.bufpool, name, data)
		if err != nil {
			return err
		}
		return nil
	}
	// ...otherwise generate ad-hoc template and cache it
	cacheEntry, err := ts.common.Clone()
	if err != nil {
		return err
	}
	// NOTE: since Clone() does not clone options, should I re-configure .Option() here?
	for _, nm := range names {
		tmpl, _ := lookup(ts, nm)
		if tmpl == nil {
			return fmt.Errorf("No such template '%s'\n", nm)
		}
		err := addParseTree(cacheEntry, tmpl)
		if err != nil {
			return err
		}
	}
	ts.cache[fullname] = cacheEntry
	err = executeTemplate(cacheEntry, w, ts.bufpool, name, data)
	if err != nil {
		return err
	}
	return nil
}
