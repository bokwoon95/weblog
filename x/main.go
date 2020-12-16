package main

import (
	"html/template"
	"log"
	"os"
)

type fmap = map[string]interface{}

var t1 = template.Must(template.New("t1").Funcs(t1_funcs).Parse(`<div>tis t1: {{ t1 }} {{ template "potash" }}</div>
{{ define "potash" }}
ashesburnit
{{ end }}
`))

var t1_funcs = fmap{
	"t1": func() string { return "t1" },
}

var t2 = template.Must(template.New("t2").Funcs(t2_funcs).Parse("<div>tis t2: {{ t2 }}</div>"))

var t2_funcs = fmap{
	"t2": func() string { return "t2" },
}

var all_text = `<div>tis all</div>
{{ template "mccree" }}
{{ template "t2" }}
{{ t1 }}
{{ t2 }}`

var all_funcs = fmap{
	"oozora": func() string { return "subaru" },
	"t1":     func() string { return "butcharu" },
}

func addParseTree(parent, child *template.Template, childName string) error {
	var err error
	for _, t := range child.Templates() {
		if t == child {
			name := t.Name()
			if childName != "" {
				name = childName
			}
			_, err = parent.AddParseTree(name, t.Tree)
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

func main() {
	var err error
	t := template.New("")
	err = addParseTree(t, t1, "mccree")
	if err != nil {
		log.Fatalln(err)
	}
	err = addParseTree(t, t2, t2.Name())
	if err != nil {
		log.Fatalln(err)
	}
	t, err = t.New("all").Funcs(combine(all_funcs, t1_funcs, t2_funcs)).Parse(all_text)
	if err != nil {
		log.Fatalln(err)
	}
	err = t.Execute(os.Stdout, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func combine(mps ...fmap) fmap {
	out := fmap{}
	for _, mp := range mps {
		for k, v := range mp {
			out[k] = v
		}
	}
	return out
}
