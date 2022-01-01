package main

import "time"

type User struct {
	LastPost    string
	Follows     []string
	DisplayName string
	PublicName  string //ens or dns name
}

type Post struct {
	Previous string
	Content  string
	Created  time.Time //can't actually trust this
}

type FetchedPost struct {
	Post
	RenderedContent  string
	Author           string
	AuthorPublicName string
}

const nobody = "nobody"
