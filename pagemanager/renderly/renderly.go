package renderly

import (
	"crypto/sha256"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/bokwoon95/weblog/pagemanager/erro"
	"github.com/oxtoacart/bpool"
)

type Renderly struct {
	mu      *sync.RWMutex
	bufpool *bpool.BufferPool
	fs      fs.FS
	altfs   map[string]fs.FS
	funcs   map[string]interface{}
	opts    []string
	// plugin
	html      *template.Template
	css       map[string][]*Asset
	js        map[string][]*Asset
	prehooks  map[string][]Prehook
	posthooks map[string][]Posthook
	// fs cache
	cacheenabled bool
	cachepage    map[string]Page
	cachehtml    map[string]*template.Template
	cachecss     map[string]*Asset
	cachejs      map[string]*Asset
	//
	errorhandler func(http.ResponseWriter, *http.Request, error)
}

type Asset struct {
	Data     string
	Hash     [32]byte
	External bool
}

type Prehook func(w io.Writer, r *http.Request, input interface{}) (output interface{}, err error)

type Posthook func(io.Writer, *http.Request) error

func New(fsys fs.FS, opts ...Option) (*Renderly, error) {
	ry := &Renderly{
		mu:      &sync.RWMutex{},
		fs:      fsys,
		altfs:   make(map[string]fs.FS),
		bufpool: bpool.NewBufferPool(64),
		funcs:   make(map[string]interface{}),
		// plugin
		html:      template.New(""),
		css:       make(map[string][]*Asset),
		js:        make(map[string][]*Asset),
		prehooks:  make(map[string][]Prehook),
		posthooks: make(map[string][]Posthook),
		// fs cache
		cachepage: make(map[string]Page),
		cachehtml: make(map[string]*template.Template),
		cachecss:  make(map[string]*Asset),
		cachejs:   make(map[string]*Asset),
	}
	var err error
	for _, opt := range opts {
		err = opt(ry)
		if err != nil {
			return ry, err
		}
	}
	return ry, nil
}

func (ry *Renderly) Page(w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) error {
	page, err := ry.Lookup(filenames...)
	if err != nil {
		return erro.Wrap(err)
	}
	err = page.Render(w, r, data)
	if err != nil {
		return erro.Wrap(err)
	}
	return nil
}

func (ry *Renderly) InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	if ry.errorhandler != nil {
		ry.errorhandler(w, r, err)
		return
	}
	io.WriteString(w, erro.Sdump(err))
}

type Option func(*Renderly) error

func TemplateFuncs(funcmaps ...map[string]interface{}) Option {
	return func(ry *Renderly) error {
		if ry.funcs == nil {
			ry.funcs = make(map[string]interface{})
		}
		for _, funcmap := range funcmaps {
			for name, fn := range funcmap {
				ry.funcs[name] = fn
			}
		}
		return nil
	}
}

func TemplateOpts(option ...string) Option {
	return func(ry *Renderly) error {
		ry.opts = option
		return nil
	}
}

func GlobalCSS(fsys fs.FS, filenames ...string) Option {
	return func(ry *Renderly) error {
		for _, name := range filenames {
			b, err := fs.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			ry.css[""] = append(ry.css[""], &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			})
		}
		return nil
	}
}

func GlobalJS(fsys fs.FS, filenames ...string) Option {
	return func(ry *Renderly) error {
		for _, name := range filenames {
			b, err := fs.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			ry.js[""] = append(ry.js[""], &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			})
		}
		return nil
	}
}

func GlobalTemplates(fsys fs.FS, filenames ...string) Option {
	return func(ry *Renderly) error {
		if ry.html == nil {
			ry.html = template.New("")
		}
		for _, name := range filenames {
			b, err := fs.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			t, err := template.New(name).Funcs(ry.funcs).Option(ry.opts...).Parse(string(b))
			if err != nil {
				return err
			}
			err = addParseTree(ry.html, t, t.Name())
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func AltFS(name string, fsys fs.FS) Option {
	return func(ry *Renderly) error {
		ry.altfs[name] = fsys
		return nil
	}
}

func AbsDir(relativePath string) string {
	_, absolutePath, _, _ := runtime.Caller(1)
	return filepath.Join(absolutePath, "..", relativePath) + string(os.PathSeparator)
}

type Plugin struct {
	HTML      *template.Template
	CSS       []*Asset
	JS        []*Asset
	Prehooks  []Prehook
	Posthooks []Posthook
	// global assets
	GlobalCSS       []*Asset
	GlobalJS        []*Asset
	GlobalPrehooks  []Prehook
	GlobalPosthooks []Posthook
}

func Plugins(plugins ...Plugin) Option {
	return func(ry *Renderly) error {
		if ry.html == nil {
			ry.html = template.New("")
		}
		for _, plugin := range plugins {
			name := plugin.HTML.Name()
			err := addParseTree(ry.html, plugin.HTML, name)
			if err != nil {
				return err
			}
			// Compute the hash for each asset
			for _, assets := range [][]*Asset{plugin.CSS, plugin.JS, plugin.GlobalCSS, plugin.GlobalJS} {
				for _, asset := range assets {
					asset.Hash = sha256.Sum256([]byte(asset.Data))
				}
			}
			ry.css[name] = append(ry.css[name], plugin.CSS...)
			ry.js[name] = append(ry.js[name], plugin.JS...)
			ry.prehooks[name] = append(ry.prehooks[name], plugin.Prehooks...)
			ry.posthooks[name] = append(ry.posthooks[name], plugin.Posthooks...)
			ry.css[""] = append(ry.css[""], plugin.GlobalCSS...)
			ry.js[""] = append(ry.js[""], plugin.GlobalJS...)
			ry.prehooks[""] = append(ry.prehooks[""], plugin.GlobalPrehooks...)
			ry.posthooks[""] = append(ry.posthooks[""], plugin.GlobalPosthooks...)
		}
		return nil
	}
}
