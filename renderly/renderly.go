package renderly

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/oxtoacart/bpool"
)

type Renderly struct {
	mu      *sync.RWMutex
	bufpool *bpool.BufferPool
	fs      FS
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
}

type Asset struct {
	Data string
	Hash [32]byte
}

type Prehook func(w http.ResponseWriter, r *http.Request, input interface{}) (output interface{}, err error)

type Posthook func(http.ResponseWriter, *http.Request) error

func New(fs FS, opts ...Option) (*Renderly, error) {
	rn := &Renderly{
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

func (rn *Renderly) Page(w http.ResponseWriter, r *http.Request, data interface{}, name string, names ...string) error {
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
