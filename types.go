package main

import "time"

type User struct {
	LastPost    string
	Follows     []string
	DisplayName string
}

type Post struct {
	Previous string
	Content  string
	Created  time.Time //can't actually trust this
}

type FetchedPost struct {
	Post
	RenderedContent   string
	Author            string
	AuthorDisplayName string
}

const nobody = "nobody"
