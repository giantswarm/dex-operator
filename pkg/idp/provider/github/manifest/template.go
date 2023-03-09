package manifest

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

var (
	//go:embed static
	embedded embed.FS
)

type Template struct {
	URL     string
	Content string
}

type Renderer struct {
	fs fs.FS
}

func newRenderer() *Renderer {
	fs := http.FileServer(http.FS(embedded))
	http.Handle("/static/", noCache(fs))
	return &Renderer{fs: embedded}
}

func (r *Renderer) Render(location string, w http.ResponseWriter, data any) error {
	tpl, err := template.ParseFS(r.fs, fmt.Sprintf("static/%s", location), "static/layout.tmpl")
	if err != nil {
		return err
	}
	return tpl.ExecuteTemplate(w, "layout", data)
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
		h.ServeHTTP(w, r)
	})
}
