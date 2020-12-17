package renderly

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/oxtoacart/bpool"
)

type Asset struct {
	Data string
	Hash [32]byte
}

type Prehook func(w http.ResponseWriter, r *http.Request, input interface{}) (output interface{}, err error)

type Posthook func(http.ResponseWriter, *http.Request) error

type Render struct {
	mu      *sync.RWMutex
	bufpool *bpool.BufferPool
	// user-provided
	fs        FS
	funcs     map[string]interface{}
	opts      []string
	prehooks  []Prehook
	posthooks []Posthook
	// plugin-provided
	base            *template.Template
	plugincss       map[string][]*Asset
	pluginjs        map[string][]*Asset
	pluginprehooks  map[string][]Prehook
	pluginposthooks map[string][]Posthook
	// cache
	cacheenabled bool
	cachepage    map[string]Page
	cachehtml    map[string]*template.Template
	cachecss     map[string]*Asset
	cachejs      map[string]*Asset
}

type Option func(*Render) error

func TFuncs(funcs map[string]interface{}) Option {
	return func(rn *Render) error {
		rn.funcs = funcs
		return nil
	}
}

func TOpts(option ...string) Option {
	return func(rn *Render) error {
		rn.opts = option
		return nil
	}
}

func New(fs FS, opts ...Option) (*Render, error) {
	rn := &Render{
		fs: fs,
	}
	var err error
	for _, opt := range opts {
		err = opt(rn)
		if err != nil {
			return rn, err
		}
	}
	return rn, nil
}

func (rn *Render) Page(w http.ResponseWriter, r *http.Request, data interface{}, name string, names ...string) error {
	page, err := rn.Lookup(name, names...)
	if err != nil {
		return err
	}
	err = page.Render(w, r, data)
	if err != nil {
		return err
	}
	return nil
}
