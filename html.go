package main

import (
	"embed"
	"html/template"
)

//go:embed templates
var content embed.FS

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(content, "templates/*.tmpl")
}
