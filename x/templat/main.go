package main

import "html/template"

// original idea: all templat plugins provide a Funcs(in)out method, templat will group them together at parse time

// natural extension: how to share templates and funcmaps across pagemanager plugins?
// - templat plugins share templates and funcmaps by being defined together. this is where the css and js get defined.
// - pagemanager plugins will benefit from being able to access pagemanager's common set of funcs and templates.
//		so the real question is how to extend a templat templates. The problem is that it will def require an entire reparse because functions annoyingly can only be defined at parse time. You can't just addParseTree to the problem

var f1 = map[string]interface{}{
	"base1": func() string { return "b1" },
	"base2": func() string { return "b2" },
	"base3": func() string { return "b3" },
}

var f2 = map[string]interface{}{
	"extend1": func() string { return "e1" },
	"extend2": func() string { return "e2" },
}

var t1 = template.Must(template.
	New("t1").
	Funcs(f1).
	Parse(`{{ base1 }} {{ base2 }} {{ base3 }}`),
)

func main() {
}
