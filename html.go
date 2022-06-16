package main

import "html/template"

var indextmpl string = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{ .UserPublicName }}</title>
	</head>
	<body>
		<div><a href="/user/{{ .UserPublicName }}">{{ .UserPublicName }}</a>(ask friends to follow {{ .UserPublicName }})</div>
		<br />
		<form action="/post" method="post">
			<textarea name="post" rows="12" cols="100"></textarea>
			<br/>
			<input type="file" name="images" accept="image/*" multiple="true">
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
		{{range .Images}}
		<img src="/img/{{.}}"/>
		{{end}}
		<div><a href="/user/{{ .AuthorPublicName }}">{{ .AuthorPublicName }}</a> at {{ .Created }}</div>
		<br />		
        {{else}}
        <div><strong>No Posts</strong></div>
        {{end}}
	</body>
</html>`

var registertmpl string = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>Register public name</title>
	</head>
	<body>
		<div>Claim a domain on northbriton.net</div>
		<br />
		<form action="/register" method="post">
			<textarea name="publicname" rows="2" cols="64"></textarea>
			<br/>
			<input type="submit" value="Submit">
		</form>
		<br/>
		<div>Todo let people claim an existing domain/ens and verify it</div>
	</body>
</html>`

//https://pkg.go.dev/github.com/gin-gonic/gin#readme-build-a-single-binary-with-templates
func loadTemplates() (*template.Template, error) {
	t, err := template.New("index.tmpl").Parse(indextmpl)
	if err != nil {
		return nil, err
	}
	return t.New("register.tmpl").Parse(registertmpl)
}
