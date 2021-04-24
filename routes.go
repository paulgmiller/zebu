package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func userfeed(backend Backend, c *gin.Context) {
	me, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
		return
	}
	var followedposts []FetchedPost
	for _, follow := range me.Follows {
		f, err := backend.GetUserById(follow)
		if err != nil {
			errorPage(err, c)
			return
		}
		posts, err := backend.GetPosts(f, 10)
		if err != nil {
			errorPage(err, c)
			return
		}
		for _, p := range posts {
			content, err := backend.Cat(p.Content)
			if err != nil {
				errorPage(err, c)
				return
			}
			followedposts = append(followedposts, FetchedPost{
				Post:            p,
				RenderedContent: string(content),
				Author:          follow,
			})
		}
	}
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts": followedposts,
		"Me":    backend.GetUserId(),
	})
}

func errorPage(err error, c *gin.Context) {
	c.JSON(400, gin.H{"msg": err})
}

type simpleuser struct {
	Id string `uri:"id" binding:"required"`
}

func userpage(backend Backend, c *gin.Context) {
	var simpleUser simpleuser
	if err := c.ShouldBindUri(&simpleUser); err != nil {
		errorPage(err, c)
		return
	}

	if simpleUser.Id == "" {
		//this didn't work
		errorPage(fmt.Errorf("no user supplied"), c)
		return
	}
	log.Printf("getting user %s", simpleUser.Id)
	user, err := backend.GetUserById(simpleUser.Id)
	if err != nil {
		errorPage(err, c)
		return
	}

	userPosts(backend, user, simpleUser.Id, c)
}

func userPosts(backend Backend, user User, author string, c *gin.Context) {
	var userposts []FetchedPost
	posts, err := backend.GetPosts(user, 10)
	if err != nil {
		errorPage(err, c)
		return
	}
	for _, p := range posts {
		content, err := backend.Cat(p.Content)
		if err != nil {
			errorPage(err, c)
			return
		}
		userposts = append(userposts, FetchedPost{
			Post:            p,
			RenderedContent: string(content),
			Author:          author,
		})
	}
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts": userposts,
		"Me":    author, //a little wierd
	})
}

type simplePost struct {
	Post string `form:"post"`
}

func acceptPost(backend Backend, c *gin.Context) {
	me, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
		return
	}

	var simplePost simplePost
	c.Bind(&simplePost)
	if simplePost.Post == "" {
		log.Printf("form: %v", c.Request.Form)
		errorPage(fmt.Errorf("Empty post"), c)
		return
	}

	cid, err := backend.Add(simplePost.Post)
	if err != nil {
		errorPage(err, c)
		return
	}

	post := Post{
		Previous: me.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
	}
	err = backend.SavePost(post, me)
	if err != nil {
		errorPage(err, c)
		return
	}

	c.Redirect(http.StatusFound, "/user/"+backend.GetUserId())
}
