package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

func serve(backend Backend) {
	router := gin.Default()
	//https://gin-gonic.com/docs/examples/bind-single-binary-with-template/
	t, err := loadTemplate()
	if err != nil {
		log.Fatalf("couldnt load template, %s", err)
	}
	router.SetHTMLTemplate(t)
	router.GET("/", func(c *gin.Context) {
		userfeed(backend, c)
	})
	router.POST("/post", func(c *gin.Context) {
		acceptPost(backend, c)
	})
	router.POST("/follow", func(c *gin.Context) {
		acceptFollow(backend, c)
	})
	router.GET("/user/:id", func(c *gin.Context) {
		userpage(backend, c)
	})
	log.Print(router.Run(":9000").Error())
}

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
				Post:              p,
				RenderedContent:   string(content),
				Author:            follow,
				AuthorDisplayName: f.DisplayName,
			})
		}
	}
	//users could lie abotu time but trust for now
	sort.Slice(followedposts, func(i, j int) bool { return followedposts[i].Created.After(followedposts[j].Created) })
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts": followedposts,
		"Me":    backend.GetUserId(),
	})
}

func errorPage(err error, c *gin.Context) {
	log.Printf("ERROR: %s", err.Error())
	c.JSON(400, gin.H{"msg": err.Error()})
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
			Post:              p,
			RenderedContent:   string(content),
			Author:            author,
			AuthorDisplayName: user.DisplayName,
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

type simpleFollow struct {
	Followee string `form:"followee"`
}

func acceptFollow(backend Backend, c *gin.Context) {
	user, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
	}

	var simpleFollow simpleFollow
	c.Bind(&simpleFollow)
	user.Follows = append(user.Follows, simpleFollow.Followee)
	backend.SaveUser(user)

	c.Redirect(http.StatusFound, "/user/"+simpleFollow.Followee)
}
