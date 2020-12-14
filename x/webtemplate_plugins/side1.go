package main

import "github.com/bokwoon95/weblog/webtemplate"

const side1_html = `{{ define "side1" }}
<div class="side1">side1</div>
{{ end }}`

const side1_css = `.side1 {
  display: block;
}`

const side1_js = `window.side1 = "side1";`

func Side1(srcs *webtemplate.Sources) error {
	return nil
}
