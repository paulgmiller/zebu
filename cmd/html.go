package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates
var content embed.FS

//go:embed static
var static embed.FS

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(content, "templates/*.tmpl")
}

func loadStatic() http.FileSystem {
	noprefix, err := fs.Sub(static, "static")
	if err != nil {
		panic("couldnt load static files")
	}
	return http.FS(noprefix)
}
