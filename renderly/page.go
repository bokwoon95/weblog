package renderly

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"text/template/parse"

	"github.com/oxtoacart/bpool"
)

type Page struct {
	bufpool   *bpool.BufferPool
	html      *template.Template
	css       []*Asset
	js        []*Asset
	prehooks  []Prehook
	posthooks []Posthook
}

func (rn *Render) Lookup(name string, names ...string) (Page, error) {
	fullname := strings.Join(append([]string{name}, names...), "\n")
	// If page is already cached for the given fullname, return that page and exit
	if rn.cacheenabled {
		rn.mu.RLock()
		page, ok := rn.cachepage[fullname]
		rn.mu.RUnlock()
		if ok {
			return page, nil
		}
	}
	var err error
	// Else construct the page from scratch
	page := Page{
		bufpool:   rn.bufpool,
		prehooks:  rn.prehooks,
		posthooks: rn.posthooks,
	}
	// Clone the page template from the base template
	page.html, err = rn.base.Clone()
	if err != nil {
		return page, err
	}
	page.html = page.html.Option(rn.opts...)
	HTML, CSS, JS := categorize(append([]string{name}, names...))
	// Add user-specified HTML templates to the page template
	for _, Name := range HTML {
		var t *template.Template
		// If the template is already cached for the given file Name, use that template
		if rn.cacheenabled {
			rn.mu.RLock()
			t = rn.cachehtml[Name]
			rn.mu.RUnlock()
		}
		// Else construct the template from scratch
		if t == nil {
			b, err := ReadFile(rn.fs, Name)
			if err != nil {
				return page, err
			}
			t, err = template.New(Name).Funcs(rn.funcs).Option(rn.opts...).Parse(string(b))
			if err != nil {
				return page, err
			}
			// Cache the template if the user enabled it
			if rn.cacheenabled {
				rn.mu.Lock()
				rn.cachehtml[Name] = t
				rn.mu.Unlock()
			}
		}
		// Add to page template
		err := addParseTree(page.html, t, t.Name())
		if err != nil {
			return page, err
		}
	}
	// Find the list of dependency templates invoked by the `name` template
	depedencies, err := listAllDeps(page.html, name)
	if err != nil {
		return page, err
	}
	// For each depedency template, figure out the corresponding set of
	// CSS/JS/Prehooks/Posthooks to include in the page. A map is used keep
	// track of every included CSS/JS asset (identified by their hash) so that
	// we do not include the same asset twice.
	cssset := make(map[[32]byte]struct{})
	jsset := make(map[[32]byte]struct{})
	for _, templateName := range depedencies {
		for _, asset := range rn.plugincss[templateName] {
			if _, ok := cssset[asset.Hash]; ok {
				continue
			}
			cssset[asset.Hash] = struct{}{}
			page.css = append(page.css, asset)
		}
		for _, asset := range rn.pluginjs[templateName] {
			if _, ok := jsset[asset.Hash]; ok {
				continue
			}
			jsset[asset.Hash] = struct{}{}
			page.js = append(page.js, asset)
		}
		page.prehooks = append(page.prehooks, rn.pluginprehooks[templateName]...)
		page.posthooks = append(page.posthooks, rn.pluginposthooks[templateName]...)
	}
	// Add the user-specified CSS files to the page
	for _, Name := range CSS {
		var asset *Asset
		// If CSS asset is already cached for the given file name, use that asset
		if rn.cacheenabled {
			rn.mu.RLock()
			asset = rn.cachecss[Name]
			rn.mu.RUnlock()
		}
		// Else construct the CSS asset from scratch
		if asset == nil {
			b, err := ReadFile(rn.fs, Name)
			if err != nil {
				return page, err
			}
			asset = &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			}
			// Cache the CSS asset if the user enabled it
			if rn.cacheenabled {
				rn.mu.Lock()
				rn.cachecss[Name] = asset
				rn.mu.Unlock()
			}
		}
		// Add CSS asset to page if it hasn't already been added
		if _, ok := cssset[asset.Hash]; !ok {
			cssset[asset.Hash] = struct{}{}
			page.css = append(page.css, asset)
		}
	}
	// Add the user-specified JS files to the page
	for _, Name := range JS {
		var asset *Asset
		// If JS asset is already cached for the given file name, use that asset
		if rn.cacheenabled {
			rn.mu.RLock()
			asset = rn.cachejs[Name]
			rn.mu.RUnlock()
		}
		// Else construct the JS asset from scratch
		if asset == nil {
			b, err := ReadFile(rn.fs, Name)
			if err != nil {
				return page, err
			}
			asset = &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			}
			// Cache the JS asset if the user enabled it
			if rn.cacheenabled {
				rn.mu.Lock()
				rn.cachejs[Name] = asset
				rn.mu.Unlock()
			}
		}
		// Add JS asset to page if it hasn't already been added
		if _, ok := jsset[asset.Hash]; !ok {
			jsset[asset.Hash] = struct{}{}
			page.js = append(page.js, asset)
		}
	}
	// Cache the page if the user enabled it
	if rn.cacheenabled {
		rn.mu.Lock()
		rn.cachepage[fullname] = page
		rn.mu.Unlock()
	}
	return page, nil
}

func (page Page) Render(w http.ResponseWriter, r *http.Request, data interface{}) error {
	var err error
	for _, fn := range page.prehooks {
		data, err = fn(w, r, data)
		if err != nil {
			return err
		}
	}
	// Write Content-Security-Policy style-src-elem
	styles := &strings.Builder{}
	styleHashes := &strings.Builder{}
	for i, asset := range page.css {
		if i > 0 {
			styles.WriteString("\n")
			styleHashes.WriteString(" ")
		}
		styles.WriteString("<style>")
		styles.WriteString(asset.Data)
		styles.WriteString("</style>")
		styleHashes.WriteString("'sha256-")
		styleHashes.WriteString(hex.EncodeToString(asset.Hash[0:]))
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
		scripts.WriteString(asset.Data)
		scripts.WriteString("</script>")
		scriptHashes.WriteString("'sha256-")
		scriptHashes.WriteString(hex.EncodeToString(asset.Hash[0:]))
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
	err = executeTemplate(page.html, page.bufpool, w, page.html.Name(), data)
	if err != nil {
		return err
	}
	for _, fn := range page.posthooks {
		err = fn(w, r)
		if err != nil {
			return err
		}
	}
	return nil
}

func listAllDeps(t *template.Template, name string) ([]string, error) {
	var allnames = []string{t.Name()}
	var set = make(map[string]struct{})
	var root parse.Node = t.Tree.Root
	var roots []parse.Node
	t = t.Lookup(name) // set the main template to `name`
	if t == nil {
		return allnames, fmt.Errorf(`no such template "%s"`, name)
	}
	for {
		names := listDeps(root)
		for _, name := range names {
			if _, ok := set[name]; ok {
				continue
			}
			set[name] = struct{}{}
			allnames = append(allnames, name)
			t = t.Lookup(name)
			if t == nil {
				return allnames, fmt.Errorf(`{{ template "%s" }} was referenced, but was not found`, name)
			}
			roots = append(roots, t.Tree.Root)
		}
		if len(roots) == 0 {
			break
		}
		root, roots = roots[0], roots[1:]
	}
	return allnames, nil
}

func listDeps(node parse.Node) []string {
	var names []string
	switch node := node.(type) {
	case *parse.TemplateNode:
		names = append(names, node.Name)
	case *parse.ListNode:
		for _, n := range node.Nodes {
			names = append(names, listDeps(n)...)
		}
	}
	return names
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

func categorize(names []string) (html, css, js []string) {
	for _, name := range names {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".css":
			css = append(css, name)
		case ".js":
			js = append(js, name)
		default:
			html = append(html, name)
		}
	}
	return html, css, js
}

func addParseTree(parent, child *template.Template, childName string) error {
	var err error
	if childName == "" {
		childName = child.Name()
	}
	for _, t := range child.Templates() {
		if t == child {
			_, err = parent.AddParseTree(childName, t.Tree)
			if err != nil {
				return err
			}
			continue
		}
		_, err = parent.AddParseTree(t.Name(), t.Tree)
		if err != nil {
			return err
		}
	}
	return nil
}
