package webtemplate

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template/parse"

	"github.com/oxtoacart/bpool"
)

func Directory(skip int) string {
	_, filename, _, _ := runtime.Caller(1)
	elems := []string{filepath.Dir(filename)}
	for i := 0; i < skip; i++ {
		elems = append(elems, "..")
	}
	return filepath.Join(elems...) + string(os.PathSeparator)
}

type Source struct {
	// equivalent html/template call:
	// t.New(src.Name).Funcs(src.Funcs).Option(src.Options...).Parse(src.Text)
	Name       string
	NameInText bool
	Filepaths  []string
	Text       string
	CSS        []*CSS
	JS         []*JS
	// NOTE: still not sure if template-specific funcs are a good idea. Common funcs should be dumped in srcs.CommonFuncs, all funcs should be global?
	Funcs   map[string]interface{}
	Options []string
}

type Sources struct {
	Templates       []Source
	CommonTemplates []Source
	CommonFuncs     map[string]interface{}
	CommonOptions   []string
	DataFuncs       []func(http.ResponseWriter, *http.Request, map[string]interface{})
}

type CSS struct {
	URL  string
	Text template.CSS
	Hash string
}

type JS struct {
	URL  string
	Text template.JS
	Hash string
}

type Templates struct {
	bufpool   *bpool.BufferPool
	common    *template.Template            // gets included in every template in the cache
	lib       map[string]*template.Template // never gets executed, main purpose for cloning
	cache     map[string]*template.Template // is what gets executed, should not changed after it is set
	css       map[string][]*CSS
	js        map[string][]*JS
	funcs     map[string]interface{}
	opts      []string
	datafuncs []func(http.ResponseWriter, *http.Request, map[string]interface{})
}

type OptionParse func(*Sources) error

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

func Parse(opts ...OptionParse) (*Templates, error) {
	var err error
	ts := &Templates{
		bufpool: bpool.NewBufferPool(64),
		common:  template.New(""),
		lib:     make(map[string]*template.Template),
		cache:   make(map[string]*template.Template),
		css:     make(map[string][]*CSS),
		js:      make(map[string][]*JS),
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
	ts.opts = srcs.CommonOptions // clone options
	ts.datafuncs = srcs.DataFuncs
	if len(srcs.CommonFuncs) > 0 {
		ts.common = ts.common.Funcs(srcs.CommonFuncs)
	}
	if len(srcs.CommonOptions) > 0 {
		ts.common = ts.common.Option(srcs.CommonOptions...)
	}
	for _, src := range srcs.CommonTemplates {
		if src.NameInText {
			ts.common = ts.common.New("")
		} else {
			ts.common = ts.common.New(src.Name)
		}
		ts.common, err = ts.common.Parse(src.Text)
		if err != nil {
			return ts, err
		}
		if len(src.CSS) > 0 {
			ts.css[src.Name] = append(ts.css[src.Name], src.CSS...)
		}
		if len(src.JS) > 0 {
			ts.js[src.Name] = append(ts.js[src.Name], src.JS...)
		}
	}
	for _, src := range srcs.Templates {
		var tmpl, cacheEntry *template.Template
		// TODO: is it safe to merge src.Funcs with srcs.CommonFuncs when
		// parsing here? Will it cause problems in addParseTree? This is
		// important because it allows for template-specific funcs without
		// throwing everything into the common funcs namespace.
		if src.NameInText {
			tmpl = template.New("")
		} else {
			tmpl = template.New(src.Name)
		}
		tmpl, err = tmpl.Funcs(srcs.CommonFuncs).Option(srcs.CommonOptions...).Parse(src.Text)
		if err != nil {
			return ts, err
		}
		if len(src.CSS) > 0 {
			ts.css[src.Name] = append(ts.css[src.Name], src.CSS...)
		}
		if len(src.JS) > 0 {
			ts.js[src.Name] = append(ts.js[src.Name], src.JS...)
		}
		ts.lib[src.Name] = tmpl
		cacheEntry, err = ts.common.Clone()
		if err != nil {
			return ts, err
		}
		cacheEntry = cacheEntry.Option(srcs.CommonOptions...)
		err = addParseTree(cacheEntry, tmpl)
		if err != nil {
			return ts, err
		}
		ts.cache[src.Name] = cacheEntry
	}
	return ts, nil
}

func AddParse(base *Templates, opts ...OptionParse) (*Templates, error) {
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
		libTmpl, err := tmpl.Clone()
		if err != nil {
			return ts, err
		}
		ts.lib[name] = libTmpl
		cacheEntry, err := ts.common.Clone()
		if err != nil {
			return ts, err
		}
		cacheEntry = cacheEntry.Option(base.opts...) // clone options
		err = addParseTree(cacheEntry, libTmpl)
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
		cacheEntry = cacheEntry.Option(srcs.CommonOptions...)
		err = addParseTree(cacheEntry, tmpl)
		if err != nil {
			return ts, err
		}
		ts.cache[src.Name] = cacheEntry
	}
	return ts, nil
}

func AddCommonFiles(root string, filepatterns ...string) OptionParse {
	return func(srcs *Sources) error {
		for _, filepattern := range filepatterns {
			filenames, err := filepath.Glob(root + filepattern)
			if err != nil {
				return err
			}
			if len(filenames) == 0 {
				return fmt.Errorf("no files matching %s%s", root, filepattern)
			}
			for _, filename := range filenames {
				filename = strings.TrimPrefix(filename, root)
				src := Source{}
				b, err := ioutil.ReadFile(root + filename)
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
				srcs.CommonTemplates = append(srcs.CommonTemplates, src)
			}
		}
		return nil
	}
}

func AddFiles(root string, filepatterns ...string) OptionParse {
	return func(srcs *Sources) error {
		for _, filepattern := range filepatterns {
			filenames, err := filepath.Glob(root + filepattern)
			if err != nil {
				return err
			}
			if len(filenames) == 0 {
				return fmt.Errorf("no files matching %s%s", root, filepattern)
			}
			for _, filename := range filenames {
				filename = strings.TrimPrefix(filename, root)
				src := Source{}
				b, err := ioutil.ReadFile(root + filename)
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

func AddSources(src ...Source) OptionParse {
	return func(srcs *Sources) error {
		srcs.Templates = append(srcs.Templates, src...)
		return nil
	}
}

func AddCommonSources(src ...Source) OptionParse {
	return func(srcs *Sources) error {
		srcs.CommonTemplates = append(srcs.CommonTemplates, src...)
		return nil
	}
}

func Funcs(funcs map[string]interface{}) OptionParse {
	return func(srcs *Sources) error {
		for name, fn := range funcs {
			srcs.CommonFuncs[name] = fn
		}
		return nil
	}
}

func Option(opts ...string) OptionParse {
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
	if data == nil {
		data = make(map[string]interface{})
	}
	for _, datafunc := range ts.datafuncs {
		datafunc(w, r, data)
	}
	fullname := strings.Join(append([]string{name}, names...), "\n")
	var jsList []JS
	var cssList []CSS
	cssSet, jsSet := make(map[*CSS]bool), make(map[*JS]bool)
	invokedTemplates, err := ts.InvokedTemplates(fullname)
	if err != nil {
		return err
	}
	for _, invokedTemplate := range invokedTemplates {
		for _, css := range ts.css[invokedTemplate] {
			if cssSet[css] {
				continue
			}
			cssSet[css] = true
			cssList = append(cssList, *css)
		}
		for _, js := range ts.js[invokedTemplate] {
			if jsSet[js] {
				continue
			}
			jsSet[js] = true
			jsList = append(jsList, *js)
		}
	}
	data["__css__"] = cssList
	data["__js__"] = jsList
	// used cached version if exists...
	if cacheEntry, ok := ts.cache[fullname]; ok {
		err := executeTemplate(cacheEntry, w, ts.bufpool, name, data)
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
	cacheEntry = cacheEntry.Option(ts.opts...)
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

func (ts *Templates) InvokedTemplates(name string) ([]string, error) {
	var names []string
	tmpl := ts.cache[name]
	if tmpl == nil {
		tmpl, _ = lookup(ts, name)
		if tmpl == nil {
			return names, fmt.Errorf("no template called '%s'", name)
		}
	}
	tmpl = tmpl.Lookup(name)
	if tmpl == nil {
		return names, fmt.Errorf("Lookup() failed")
	}
	names = append(names, tmpl.Name())
	var nameSet = make(map[string]bool)
	var root parse.Node = tmpl.Tree.Root
	var roots []parse.Node
	for {
		for _, name := range listTemplates(root) {
			if !nameSet[name] {
				if t := tmpl.Lookup(name); t != nil {
					roots = append(roots, t.Tree.Root)
				}
				nameSet[name] = true
				names = append(names, name)
			}
		}
		if len(roots) == 0 {
			break
		}
		root, roots = roots[0], roots[1:]
	}
	return names, nil
}

func listTemplates(node parse.Node) []string {
	var names []string
	switch node := node.(type) {
	case *parse.TemplateNode:
		names = append(names, node.Name)
	case *parse.ListNode:
		for _, n := range node.Nodes {
			names = append(names, listTemplates(n)...)
		}
	}
	return names
}
