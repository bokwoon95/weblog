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
	return nil
}
