package main

import "github.com/bokwoon95/weblog/webtemplate"

const side1_html = `{{ define "side1" }}
<div class="side1">side1</div>
{{ template "side1:helper" . }}
{{ template "side1:helper" . }}
{{ template "side1:helper" . }}
{{ end }}
{{ define "side1:helper" }},side1:helper{{ end }}
`

const side1_css = `.side1 {
  display: block;
}`

const side1_js = `window.side1 = "side1";`

func Side1(srcs *webtemplate.Sources) error {
	src := webtemplate.Source{
		Name: "",
		Text: side1_html,
		CSS: []*webtemplate.CSS{
			{Text: side1_css},
		},
		JS: []*webtemplate.JS{
			{Text: side1_js},
		},
	}
	srcs.CommonTemplates = append(srcs.CommonTemplates, src)
	return nil
}
