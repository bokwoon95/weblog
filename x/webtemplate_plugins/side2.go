package main

import "github.com/bokwoon95/weblog/webtemplate"

const side2_html = `{{ define "side2" }}
<div class="side2">side2</div>
{{ end }}`

const side2_css = `.side2 {
  display: block;
}`

const side2_js = `window.side2 = "side2";`

func Side2(srcs *webtemplate.Sources) error {
	src := webtemplate.Source{
		Name:       "side2",
		NameInText: true,
		Text:       side2_html,
		CSS: []*webtemplate.CSS{
			{Text: side2_css},
		},
		JS: []*webtemplate.JS{
			{Text: side2_js},
		},
	}
	srcs.CommonTemplates = append(srcs.CommonTemplates, src)
	srcs.CommonFuncs["side2"] = func() string { return "side2" }
	return nil
}
