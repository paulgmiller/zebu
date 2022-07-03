package main

import (
	"html/template"
	"time"
)

type User struct {
	LastPost    string
	Follows     []string
	DisplayName string
	PublicName  string //ens or dns name
}

//ugh why doesn't this exist.
func (u *User) Follow(userCidr string) {
	for _, f := range u.Follows {
		if f == userCidr {
			return
		}
	}
	u.Follows = append(u.Follows, userCidr)
}

type Post struct {
	Previous string
	Content  string
	Images   []string
	Created  time.Time //can't actually trust this
}

type FetchedPost struct {
	Post
	RenderedContent  template.HTML
	Author           string
	AuthorPublicName string
}

const nobody = "nobody"
