package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/skip2/go-qrcode"
)

func serve(backend Backend) {
	router := gin.Default()
	//https://gin-gonic.com/docs/examples/bind-single-binary-with-template/
	t, err := loadTemplates()
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
	router.POST("/register", func(c *gin.Context) {
		registerPublicName(backend, c)
	})
	router.GET("/user/:id", func(c *gin.Context) {
		userpage(backend, c)
	})
	router.GET("/key", func(c *gin.Context) {
		qrCode(backend, c)
	})
	log.Print(router.Run(":9000").Error())
}

//look may a security hole
func qrCode(backend Backend, c *gin.Context) {
	key, err := backend.ExportKey()
	if err != nil {
		errorPage(err, c)
		return
	}
	//log.Printf("dumping key: %s", hex.EncodeToString(key))
	var png []byte
	png, err = qrcode.Encode(hex.EncodeToString(key), qrcode.Low, 256)
	if err != nil {
		errorPage(err, c)
		return
	}
	contentType := "image/png"
	c.Data(http.StatusOK, contentType, png)
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
				Post:             p,
				RenderedContent:  string(content),
				Author:           follow,
				AuthorPublicName: f.PublicName,
			})
		}
	}
	//users could lie abotu time but trust for now
	sort.Slice(followedposts, func(i, j int) bool { return followedposts[i].Created.After(followedposts[j].Created) })
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts":          followedposts,
		"UserId":         backend.GetUserId(),
		"UserPublicName": me.PublicName,
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
			Post:             p,
			RenderedContent:  string(content),
			Author:           author,
			AuthorPublicName: user.PublicName,
		})
	}
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts":          userposts,
		"UserId":         author,
		"UserPublicName": user.PublicName,
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
	Followee string `form:"followee"` //ipns?
}

func acceptFollow(backend Backend, c *gin.Context) {
	user, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
	}

	var simpleFollow simpleFollow
	c.Bind(&simpleFollow)
	_, err = backend.GetUserById(simpleFollow.Followee)
	if err != nil {
		errorPage(errors.Wrap(err, "couldn't resolve followee"), c)
	}

	user.Follows = append(user.Follows, simpleFollow.Followee)
	backend.SaveUser(user)

	c.Redirect(http.StatusFound, "/user/"+simpleFollow.Followee)
}

type simpleRegister struct {
	PublicName string `form:"publicname"`
}

func registerPublicName(backend Backend, c *gin.Context) {
	var simpleRegister simpleRegister
	c.Bind(&simpleRegister)
	url := "http://registrar.northbriton.net:8000/reserve/" + simpleRegister.PublicName

	resp, err := http.Post(url, "text/plain", strings.NewReader(backend.GetUserId()))
	if err != nil {
		errorPage(err, c)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		errorPage(fmt.Errorf("Got %d : %s", resp.StatusCode, string(body)), c)
	}

	user, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
	}
	user.PublicName = simpleRegister.PublicName + ".northbriton.net"
	backend.SaveUser(user)

	c.Redirect(http.StatusFound, "/")
}
