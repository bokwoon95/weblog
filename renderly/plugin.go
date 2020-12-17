package renderly

import (
	"crypto/sha256"
	"html/template"
)

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
	return func(rn *Render) error {
		if rn.base == nil {
			rn.base = template.New("")
		}
		for _, plugin := range plugins {
			name := plugin.HTML.Name()
			err := addParseTree(rn.base, plugin.HTML, name)
			if err != nil {
				return err
			}
			// Compute the hash for each asset
			for _, assets := range [][]*Asset{plugin.CSS, plugin.JS, plugin.GlobalCSS, plugin.GlobalJS} {
				for _, asset := range assets {
					asset.Hash = sha256.Sum256([]byte(asset.Data))
				}
			}
			rn.plugincss[name] = append(rn.plugincss[name], plugin.CSS...)
			rn.pluginjs[name] = append(rn.pluginjs[name], plugin.JS...)
			rn.pluginprehooks[name] = append(rn.pluginprehooks[name], plugin.Prehooks...)
			rn.pluginposthooks[name] = append(rn.pluginposthooks[name], plugin.Posthooks...)
			rn.plugincss[""] = append(rn.plugincss[""], plugin.GlobalCSS...)
			rn.pluginjs[""] = append(rn.pluginjs[""], plugin.GlobalJS...)
			rn.pluginprehooks[""] = append(rn.pluginprehooks[""], plugin.GlobalPrehooks...)
			rn.pluginposthooks[""] = append(rn.pluginposthooks[""], plugin.GlobalPosthooks...)
		}
		return nil
	}
}
