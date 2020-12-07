package templat

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/bokwoon95/weblog/pagemanager/erro"
	"github.com/davecgh/go-spew/spew"
	"github.com/oxtoacart/bpool"
)

// TODO: The mechanism for plugins to dump their data into the global map[string]interface{} and accessing it at a known namespace is quite simple.
// Basically their template namespace can be configured beforehand, at Parse() time. That way the user and plugin author always knows what namespace they can access their variables at. The user can also choose to tweak the namespace accordingly to avoid potential template namespace conflicts.
// Then, users are encouraged to pass the dot to the template as-is without narrowing down anything, thus allowing plugin templates to access their own data at the predetermined namespace.
// This may let templates potentially snoop at user data but essentially plugins must be trusted first before they can be used.
// That way we can sidestep any trust issues by saying it's up to the user to screen for security issues before trusting the plugin.

type ctxkey int

const (
	RenderJSON ctxkey = iota
)

type Templates struct {
	bufpool *bpool.BufferPool
	funcs   map[string]interface{}
	opts    []string
	common  *template.Template
	lib     map[string]*template.Template
	cache   map[string]*template.Template
}

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

type Opt func(t *Templates)

func Funcs(funcs map[string]interface{}) func(*Templates) {
	return func(t *Templates) {
		t.funcs = funcs
	}
}

func Option(opts ...string) func(*Templates) {
	return func(t *Templates) {
		t.opts = opts
	}
}

func stripSpaces(s string) string {
	return s
}

func Parse(common []string, templates []string, opts ...Opt) (*Templates, error) {
	main := &Templates{
		bufpool: bpool.NewBufferPool(64),
		common:  template.New(""),
		lib:     make(map[string]*template.Template),
		cache:   make(map[string]*template.Template),
	}
	for _, opt := range opts {
		opt(main)
	}
	if len(main.funcs) > 0 {
		main.common = main.common.Funcs(main.funcs)
	}
	if len(main.opts) > 0 {
		main.common = main.common.Option(main.opts...)
	}
	for _, name := range common {
		files, err := filepath.Glob(name)
		if err != nil {
			return main, err
		}
		for _, file := range files {
			b, err := ioutil.ReadFile(file)
			if err != nil {
				return main, err
			}
			var t *template.Template
			re, err := regexp.Compile(`{{\s*define\s+["` + "`" + `]` + file + `["` + "`" + `]\s*}}`)
			if err != nil {
				return main, err
			}
			if re.MatchString(string(b)) {
				t = template.New("")
			} else {
				t = template.New(file)
			}
			t, err = t.Funcs(main.funcs).Option(main.opts...).Parse(string(b))
			if err != nil {
				return main, err
			}
			main.lib[file] = t
			err = addParseTree(main.common, t)
			if err != nil {
				return main, err
			}
		}
	}
	for _, name := range templates {
		files, err := filepath.Glob(name)
		if err != nil {
			return main, err
		}
		for _, file := range files {
			b, err := ioutil.ReadFile(file)
			if err != nil {
				return main, err
			}
			var t *template.Template
			re, err := regexp.Compile(`{{\s*define\s+["` + "`" + `]` + file + `["` + "`" + `]\s*}}`)
			if err != nil {
				return main, err
			}
			if re.MatchString(string(b)) {
				t = template.New("")
			} else {
				t = template.New(file)
			}
			t, err = t.Funcs(main.funcs).Option(main.opts...).Parse(string(b))
			if err != nil {
				return main, err
			}
			main.lib[file] = t
			cacheEntry, err := main.common.Clone()
			if err != nil {
				return main, err
			}
			cacheEntry = cacheEntry.Option(main.opts...)
			err = addParseTree(cacheEntry, t)
			if err != nil {
				return main, err
			}
			main.cache[file] = cacheEntry
		}
	}
	return main, nil
}

func (main *Templates) Render(w http.ResponseWriter, r *http.Request, data map[string]interface{}, name string, names ...string) error {
	renderJSON, _ := r.Context().Value(RenderJSON).(bool)
	if renderJSON {
		sanitizedData, err := sanitizeObject(data)
		if err != nil {
			return err
		}
		b, err := json.Marshal(sanitizedData)
		if err != nil {
			s := spew.Sdump(data)
			b = []byte(s)
		}
		w.Write(b)
		return nil
	}
	// "app/students/milestone_team_evaluation.html"
	mainTemplate, ok := main.lib[name]
	if !ok {
		return erro.Wrap(fmt.Errorf("No such template '%s'\n", name))
	}
	fullname := strings.Join(append([]string{name}, names...), "\n")
	// used cached version if exists...
	if t, ok := main.cache[fullname]; ok {
		err := executeTemplate(t, w, main.bufpool, name, data)
		if err != nil {
			return err
		}
		return nil
	}
	// ...otherwise generate ad-hoc template and cache it
	cacheEntry, err := main.common.Clone()
	if err != nil {
		return err
	}
	cacheEntry = cacheEntry.Option(main.opts...)
	err = addParseTree(cacheEntry, mainTemplate)
	if err != nil {
		return err
	}
	for _, nm := range names {
		t, ok := main.lib[nm]
		if !ok {
			return erro.Wrap(fmt.Errorf("No such template '%s'\n", nm))
		}
		err := addParseTree(cacheEntry, t)
		if err != nil {
			return err
		}
	}
	main.cache[fullname] = cacheEntry
	err = executeTemplate(cacheEntry, w, main.bufpool, name, data)
	if err != nil {
		// TODO: if error is related to non-existent template, attempt to find
		// the file associated with `name`.  If it exists, do an ad-hoc
		// .Parse(), add in the common templates and .Execute() it. Do not
		// cache the result, as this is the only way we can have dynamic
		// user-defined templates with recompilation. Very valuable when
		// hosting on some server, the user can tweak templates dynamically.
		// Or should this be handled by the caller instead? They can always
		// catch the error thrown from Render() and manage it on their end.
		return err
	}
	return nil
}

func (main *Templates) DefinedTemplates() string {
	buf := &strings.Builder{}
	buf.WriteString("; The defined templates are: ")
	i := 0
	for name := range main.lib {
		if i > 0 {
			buf.WriteString(", ")
		}
		i++
		buf.WriteString(`"`)
		buf.WriteString(name)
		buf.WriteString(`"`)
	}
	return buf.String()
}

func (main *Templates) Templates() []*template.Template {
	templates := make([]*template.Template, len(main.lib))
	i := 0
	for _, t := range main.lib {
		templates[i] = t
	}
	return templates
}

func ActivateJSON(activationParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			if _, ok := r.Form[activationParam]; ok {
				ctx := r.Context()
				r = r.WithContext(context.WithValue(ctx, RenderJSON, true))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sanitizeObject(object map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})
	for key, value := range object {
		switch value := value.(type) {
		case nil: // null
			output[key] = value
		case string: // string
			output[key] = value
		case int, int8, int16, int32, int64, uint, uint8,
			uint16, uint64, uintptr, float32, float64: // number
			output[key] = value
		case map[string]interface{}: // object
			v, err := sanitizeObject(value)
			if err != nil {
				return output, err
			}
			output[key] = v
		case []interface{}: // array
			v, err := sanitizeArray(value)
			if err != nil {
				return output, err
			}
			output[key] = v
		default:
			v, err := sanitizeInterface(value)
			if err != nil {
				return output, err
			}
			output[key] = v
		}
	}
	return output, nil
}

func sanitizeArray(array []interface{}) ([]interface{}, error) {
	var output []interface{}
	for _, item := range array {
		switch value := item.(type) {
		case nil: // null
			output = append(output, value)
		case string: // string
			output = append(output, value)
		case int, int8, int16, int32, int64, uint, uint8,
			uint16, uint64, uintptr, float32, float64: // number
			output = append(output, value)
		case map[string]interface{}: // object
			v, err := sanitizeObject(value)
			if err != nil {
				return output, err
			}
			output = append(output, v)
		case []interface{}: // array
			v, err := sanitizeArray(value)
			if err != nil {
				return output, err
			}
			output = append(output, v)
		default:
			v, err := sanitizeInterface(value)
			if err != nil {
				return output, err
			}
			output = append(output, v)
		}
	}
	return output, nil
}

func sanitizeInterface(v interface{}) (interface{}, error) {
	var output interface{}
	switch vv := reflect.ValueOf(v); vv.Kind() {
	case reflect.Array: // K
		output = v
	case reflect.Chan: // K
		output = v
	case reflect.Func: // ?
		return funcType(v), nil
	case reflect.Interface: // ?
		output = v
	case reflect.Map: // K,V
		output = v
	case reflect.Ptr: // K
		output = v
	case reflect.Slice: // K
		output = v
	case reflect.Struct: // K
		output = v
	case reflect.Complex64, reflect.Complex128: // unsupported
		return output, fmt.Errorf("unsupported type: complex number")
	case reflect.UnsafePointer: // unsupported
		return output, fmt.Errorf("unsupported type: unsafe.Pointer")
	case reflect.Invalid: // unsupported
		return output, fmt.Errorf("unsupported type: reflect.Invalid")
	default:
		output = v
	}
	return output, nil
}

func funcType(f interface{}) string {
	t := reflect.TypeOf(f)
	if t.Kind() != reflect.Func {
		return "<not a function>"
	}
	buf := strings.Builder{}
	buf.WriteString("func(")
	for i := 0; i < t.NumIn(); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(t.In(i).String())
	}
	buf.WriteString(")")
	if numOut := t.NumOut(); numOut > 0 {
		if numOut > 1 {
			buf.WriteString(" (")
		} else {
			buf.WriteString(" ")
		}
		for i := 0; i < t.NumOut(); i++ {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(t.Out(i).String())
		}
		if numOut > 1 {
			buf.WriteString(")")
		}
	}
	return buf.String()
}
