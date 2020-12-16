package renderx

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/bokwoon95/weblog/renderx/fs"
	"github.com/oxtoacart/bpool"
)

type asset struct {
	data string
	hash [32]byte
}

type _page struct {
	html *template.Template
	css  []*asset
	js   []*asset
}

type Render struct {
	fs          fs.FS
	funcs       map[string]interface{}
	opts        []string
	common      []string
	cache       map[string]_page
	cssdeps     map[string][]*asset // populated by third party templates, not user templates
	jsdeps      map[string][]*asset // populated by third party templates, not user templates
	prerender   []func(w http.ResponseWriter, r *http.Request, input interface{}) (output interface{}, err error)
	postrender  []func(w http.ResponseWriter, r *http.Request) error
	enablecache bool
	bufpool     *bpool.BufferPool
	mu          *sync.RWMutex
}

// TODO
func (pg _page) fillassets(cssdeps, jsdeps map[string][]*asset) error {
	// walk the html.Tree.Root, get names
	// for each name, find the corresponding css and js *asset and add it to css, js slice if not already added (use a map to track)
	// btw the map key is the asset hash
	return nil
}
func appendCSP(w http.ResponseWriter, policy, value string) error {
	const key = "Content-Security-Policy"
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

func executeTemplate(t *template.Template, bufpool *bpool.BufferPool, w io.Writer, name string, data interface{}) error {
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

func (rn *Render) renderpage(page _page, w http.ResponseWriter, r *http.Request, data interface{}, name string) error {
	var err error
	for _, fn := range rn.prerender {
		data, err = fn(w, r, data)
		if err != nil {
			return err
		}
	}
	// NOTE: the reason I'm building the styles and scripts strings from the
	// raw []*assets instead of caching the full strings is because I'm afraid
	// of memory bloat. If allocating string.Builders and looping over the
	// []*assets every render proves to be more expensive than just stuffing
	// cached data into the heap, I can change it.
	// Write Content-Security-Policy style-src-elem
	styles := &strings.Builder{}
	styleHashes := &strings.Builder{}
	for i, asset := range page.css {
		if i > 0 {
			styles.WriteString("\n")
			styleHashes.WriteString(" ")
		}
		styles.WriteString("<style>")
		styles.WriteString(asset.data)
		styles.WriteString("</style>")
		styleHashes.WriteString("'sha256-")
		styleHashes.WriteString(hex.EncodeToString(asset.hash[0:]))
		styleHashes.WriteString("'")
	}
	err = appendCSP(w, "style-src-elem", styleHashes.String())
	if err != nil {
		return err
	}
	// Write Content-Security-Policy script-src-elem
	scripts := &strings.Builder{}
	scriptHashes := &strings.Builder{}
	for i, asset := range page.js {
		if i > 0 {
			scripts.WriteString("\n")
			scriptHashes.WriteString(" ")
		}
		scripts.WriteString("<script>")
		scripts.WriteString(asset.data)
		scripts.WriteString("</script>")
		scriptHashes.WriteString("'sha256-")
		scriptHashes.WriteString(hex.EncodeToString(asset.hash[0:]))
		scriptHashes.WriteString("'")
	}
	err = appendCSP(w, "script-src-elem", scriptHashes.String())
	if err != nil {
		return err
	}
	if mapdata, ok := data.(map[string]interface{}); ok {
		mapdata["__css__"] = template.HTML(styles.String())
		mapdata["__js__"] = template.HTML(scripts.String())
		data = mapdata
	}
	err = executeTemplate(page.html, rn.bufpool, w, name, data)
	if err != nil {
		return err
	}
	for _, fn := range rn.postrender {
		err = fn(w, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func New(fs fs.FS, funcs map[string]interface{}, opts []string) *Render {
	rn := &Render{
		fs:    fs,
		funcs: funcs,
		opts:  opts,
	}
	return rn
}

// TODO
func categorize(name string, names ...string) (html, css, js []string) {
	return html, css, js
}

func withRLock(mu *sync.RWMutex, fn func()) {
	mu.RLock()
	fn()
	mu.RUnlock()
}

func withLock(mu *sync.RWMutex, fn func()) {
	mu.Lock()
	fn()
	mu.Unlock()
}

func (rn *Render) Page(w http.ResponseWriter, r *http.Request, data interface{}, name string, names ...string) error {
	var page _page
	var err error
	var ok bool
	HTML, CSS, JS := categorize(name, names...)
	fullname := strings.Join(HTML, "\n")
	withRLock(rn.mu, func() {
		page, ok = rn.cache[fullname]
	})
	if ok {
		err = rn.renderpage(page, w, r, data, name)
		if err != nil {
			return err
		}
		return nil
	}
	page.html = template.New("").Funcs(rn.funcs).Option(rn.opts...)
	for _, Name := range rn.common {
		b, err := fs.ReadFile(rn.fs, Name)
		if err != nil {
			return err
		}
		page.html, err = page.html.New("").Parse(string(b))
		if err != nil {
			return err
		}
	}
	for _, Name := range HTML {
		b, err := fs.ReadFile(rn.fs, Name)
		if err != nil {
			return err
		}
		page.html, err = page.html.New(Name).Parse(string(b))
		if err != nil {
			return err
		}
	}
	err = page.fillassets(rn.cssdeps, rn.jsdeps)
	if err != nil {
		return err
	}
	cssset := make(map[[32]byte]bool)
	for _, Name := range CSS {
		b, err := fs.ReadFile(rn.fs, Name)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(b)
		if !cssset[hash] {
			cssset[hash] = true
			page.css = append(page.css, &asset{
				data: string(b),
				hash: hash,
			})
		}
	}
	jsset := make(map[[32]byte]bool)
	for _, Name := range JS {
		b, err := fs.ReadFile(rn.fs, Name)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(b)
		if !jsset[hash] {
			jsset[hash] = true
			page.js = append(page.js, &asset{
				data: string(b),
				hash: hash,
			})
		}
	}
	if rn.enablecache {
		withLock(rn.mu, func() {
			rn.cache[fullname] = page
		})
	}
	err = rn.renderpage(page, w, r, data, name)
	if err != nil {
		return err
	}
	return nil
}
