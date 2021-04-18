package main

import "time"

type User struct {
	LastPost string
	Follows  []string
}

type Post struct {
	Previous string
	Content  string
	Created  time.Time //can't actually trust this
}

const nobody = "nobody"
