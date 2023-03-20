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
	return &Renderer{fs: embedded}
}

func (r *Renderer) Render(location string, w http.ResponseWriter, data any) error {
	tpl, err := template.ParseFS(r.fs, fmt.Sprintf("static/%s", location), "static/layout.tmpl")
	if err != nil {
		return err
	}
	return tpl.ExecuteTemplate(w, "layout", data)
}
