package renderly

import (
	"crypto/sha256"
	"html/template"
)

type Option func(*Renderly) error

func TemplateFuncs(funcmaps ...map[string]interface{}) Option {
	return func(rn *Renderly) error {
		if rn.funcs == nil {
			rn.funcs = make(map[string]interface{})
		}
		for _, funcmap := range funcmaps {
			for name, fn := range funcmap {
				rn.funcs[name] = fn
			}
		}
		return nil
	}
}

func TemplateOpts(option ...string) Option {
	return func(rn *Renderly) error {
		rn.opts = option
		return nil
	}
}

func GlobalCSS(fsys FS, name string, names ...string) Option {
	return func(rn *Renderly) error {
		for _, name := range append([]string{name}, names...) {
			b, err := ReadFile(fsys, name)
			if err != nil {
				return err
			}
			rn.css[""] = append(rn.css[""], &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			})
		}
		return nil
	}
}

func GlobalJS(fsys FS, name string, names ...string) Option {
	return func(rn *Renderly) error {
		for _, name := range append([]string{name}, names...) {
			b, err := ReadFile(fsys, name)
			if err != nil {
				return err
			}
			rn.js[""] = append(rn.js[""], &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			})
		}
		return nil
	}
}

func GlobalTemplates(fsys FS, name string, names ...string) Option {
	return func(rn *Renderly) error {
		if rn.html == nil {
			rn.html = template.New("")
		}
		for _, name := range append([]string{name}, names...) {
			b, err := ReadFile(fsys, name)
			if err != nil {
				return err
			}
			rn.js[""] = append(rn.js[""], &Asset{
				Data: string(b),
				Hash: sha256.Sum256(b),
			})
		}
		return nil
	}
}

type Plugin struct {
	Error     error
	HTML      *template.Template
	CSS       []*Asset
	JS        []*Asset
	Prehooks  []Prehook
	Posthooks []Posthook
	// global assets/hooks
	GlobalCSS       []*Asset
	GlobalJS        []*Asset
	GlobalPrehooks  []Prehook
	GlobalPosthooks []Posthook
}

func Plugins(plugins ...Plugin) Option {
	return func(rn *Renderly) error {
		if rn.html == nil {
			rn.html = template.New("")
		}
		for _, plugin := range plugins {
			if plugin.Error != nil {
				return plugin.Error
			}
			name := plugin.HTML.Name()
			err := addParseTree(rn.html, plugin.HTML, name)
			if err != nil {
				return err
			}
			// Compute the hash for each asset
			for _, assets := range [][]*Asset{plugin.CSS, plugin.JS, plugin.GlobalCSS, plugin.GlobalJS} {
				for _, asset := range assets {
					asset.Hash = sha256.Sum256([]byte(asset.Data))
				}
			}
			rn.css[name] = append(rn.css[name], plugin.CSS...)
			rn.js[name] = append(rn.js[name], plugin.JS...)
			rn.prehooks[name] = append(rn.prehooks[name], plugin.Prehooks...)
			rn.posthooks[name] = append(rn.posthooks[name], plugin.Posthooks...)
			rn.css[""] = append(rn.css[""], plugin.GlobalCSS...)
			rn.js[""] = append(rn.js[""], plugin.GlobalJS...)
			rn.prehooks[""] = append(rn.prehooks[""], plugin.GlobalPrehooks...)
			rn.posthooks[""] = append(rn.posthooks[""], plugin.GlobalPosthooks...)
		}
		return nil
	}
}
