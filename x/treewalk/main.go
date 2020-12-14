package main

import (
	"fmt"
	"log"

	"github.com/bokwoon95/weblog/webtemplate"
)

func main() {
	wt, err := webtemplate.Parse(opts...)
	if err != nil {
		log.Fatalln(err)
	}
	names, err := wt.InvokedTemplates("all_html")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(names)
}

var opts = []webtemplate.OptionParse{
	webtemplate.AddCommonSources(webtemplate.Source{
		Name: "junk1",
		Text: junk1,
	}, webtemplate.Source{
		Name: "junk2",
		Text: junk2,
	}, webtemplate.Source{
		Name: "junk3",
		Text: junk3,
	}),
	webtemplate.AddSources(webtemplate.Source{
		Name: "all_html",
		Text: all_html,
	}),
	Side1,
	Side2,
}

const all_html = `
{{ template "side1" . }}
{{ template "side2" . }}
calling side2: {{ side2 }}
this is main
`

const junk1 = ``
const junk2 = ``
const junk3 = ``

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

const side2_html = `{{ define "side2" }}
<div class="side2">side2</div>
{{ end }}`

const side2_css = `.side2 {
  display: block;
}`

const side2_js = `window.side2 = "side2";`

func Side2(srcs *webtemplate.Sources) error {
	src := webtemplate.Source{
		Name: "",
		Text: side2_html,
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
