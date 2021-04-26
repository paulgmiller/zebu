package main

import "html/template"

var indextmpl string = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{ .UserDisplayName }}</title>
	</head>
	<body>
		<div><a href="/user/{{ .UserId }}">{{ .UserDisplayName }}</a>(ask friends to follow {{ .UserId }})</div>
		<br />
		<form action="/post" method="post">
			<textarea name="post" rows="12" cols="100"></textarea>
			<br/>
			<input type="submit" value="Submit">
		</form>
		<form action="/follow" method="post">
			<textarea name="followee" rows="2" cols="64"></textarea>
			<br/>
			<input type="submit" value="Follow">
		</form>
		<br/>
		{{range .Posts}}
		<div>{{ .RenderedContent }}</div>
		<div><a href="/user/{{ .Author }}">{{ .AuthorDisplayName }}</a> at {{ .Created }}</div>
		<br />		
        {{else}}
        <div><strong>No Posts</strong></div>
        {{end}}
	</body>
</html>`

//https://pkg.go.dev/github.com/gin-gonic/gin#readme-build-a-single-binary-with-templates
func loadTemplate() (*template.Template, error) {
	return template.New("index.tmpl").Parse(indextmpl)
}
