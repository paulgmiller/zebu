package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func serve(backend Backend) {
	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}), gin.Recovery())

	//https://gin-gonic.com/docs/examples/bind-single-binary-with-template/
	t, err := loadTemplates()
	if err != nil {
		log.Fatalf("couldnt load template, %s", err)
	}
	router.SetHTMLTemplate(t)
	router.GET("/", func(c *gin.Context) {
		account, err := c.Cookie("zebu_account")
		if err == http.ErrNoCookie {
			home(backend, c)
			return
		}
		userfeed(backend, c, account)
	})

	router.GET("/rand", func(c *gin.Context) {
		home(backend, c)
	})

	router.POST("/post", func(c *gin.Context) {
		acceptPost(backend, c)
	})

	router.POST("/sign", func(c *gin.Context) {
		sign(backend, c)
	})

	router.GET("/healthz", func(c *gin.Context) {
		if !backend.Healthz() {
			errorPage(fmt.Errorf("ipfs isn't up"), c)
		}
		c.Status(200)
	})

	router.POST("/follow", func(c *gin.Context) {
		acceptFollow(backend, c)
	})

	router.POST("/register", func(c *gin.Context) {
		registerDisplayName(backend, c)
	})

	router.GET("/user/:id", func(c *gin.Context) {
		userpage(backend, c)
	})
	router.GET("/img/:cidr", func(c *gin.Context) {
		cidr := c.Param("cidr")
		imgreader, err := backend.Cat(c.Request.Context(), cidr)
		if err != nil {
			errorPage(err, c)
			return
		}
		//can't get lenth without buf.
		buf := &bytes.Buffer{}
		_, err = buf.ReadFrom(imgreader)
		if err != nil {
			errorPage(err, c)
			return
		}

		c.DataFromReader(http.StatusOK, int64(buf.Len()), "image/*", buf, map[string]string{})
	})

	router.Static("/static", "./static")

	log.Print(router.Run(":9000").Error())
}

func home(backend Backend, c *gin.Context) {
	users := backend.RandomUsers(3)
	log.Printf("getting random users %v", users)
	homeposts := []FetchedPost{}
	ctx := c.Request.Context()
	for _, u := range users {
		user, err := backend.GetUserById(ctx, u)
		if err != nil {
			errorPage(err, c)
			return
		}
		log.Printf("got user %s", user.DisplayName)
		posts, err := userPosts(ctx, backend, user, 3)
		if err != nil {
			errorPage(err, c)
			return
		}
		homeposts = append(homeposts, posts...)
	}
	sortposts(homeposts)
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts": homeposts,
	})
}

//sort by create time. users could lie abotu time but trust for now
func sortposts(posts []FetchedPost) {
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Created.After(posts[j].Created)
	})
}

//show what a user is following rahter than their posts.
func userfeed(backend Backend, c *gin.Context, account string) {
	ctx := c.Request.Context()
	me, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("got user %v", me)
	var followedposts []FetchedPost
	for _, follow := range me.Follows {
		f, err := backend.GetUserById(ctx, follow)
		if err != nil {
			errorPage(err, c)
			return
		}
		posts, err := backend.GetPosts(ctx, f, 10)
		if err != nil {
			errorPage(err, c)
			return
		}
		for _, p := range posts {
			content, err := CatString(ctx, backend, p.Content)
			if err != nil {
				errorPage(err, c)
				return
			}
			followedposts = append(followedposts, FetchedPost{
				Post:             p,
				RenderedContent:  template.HTML(content),
				Author:           follow,
				AuthorPublicName: f.DisplayName,
				//send up public name too
			})
		}
	}

	//show them random users if they have no one to follow?
	mine, err := backend.GetPosts(ctx, me, 1)
	if err != nil {
		errorPage(err, c)
		return
	}

	if len(mine) > 0 {
		p := mine[0]
		content, err := CatString(ctx, backend, p.Content)
		if err != nil {
			errorPage(err, c)
			return
		}
		followedposts = append(followedposts, FetchedPost{
			Post:             p,
			RenderedContent:  template.HTML(content),
			Author:           me.DisplayName,
			AuthorPublicName: me.PublicName,
		})
	}
	sortposts(followedposts)
	name := me.DisplayName
	if name == "" {
		name = me.PublicName
	}
	c.HTML(http.StatusOK, "user.tmpl", gin.H{
		"Posts":          followedposts,
		"UserId":         account,
		"UserPublicName": name,
	})
}

func errorPage(err error, c *gin.Context) {
	log.Printf("ERROR: %s", err.Error())
	c.JSON(500, gin.H{"msg": err.Error()})
}

type simpleuser struct {
	Id string `uri:"id" binding:"required"`
}

func userpage(backend Backend, c *gin.Context) {
	ctx := c.Request.Context()

	var simpleUser simpleuser
	if err := c.ShouldBindUri(&simpleUser); err != nil {
		errorPage(err, c)
		return
	}

	account := simpleUser.Id
	if account == "" {
		//this didn't work
		errorPage(fmt.Errorf("no user supplied"), c)
		return
	}

	log.Printf("looking up %s", account)

	//where is the best place to do this conistently.
	account, err := Resolve(account)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("resolved to %s", account)

	user, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
		return
	}

	if c.Query("raw") == "true" {
		c.IndentedJSON(http.StatusOK, user)
		return
	}

	userposts, err := userPosts(ctx, backend, user, 10)
	if err != nil {
		errorPage(err, c)
		return
	}

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts":          userposts,
		"UserId":         user.DisplayName,
		"UserPublicName": user.PublicName,
	})
}

func userPosts(ctx context.Context, backend Backend, user User, count int) ([]FetchedPost, error) {
	var userposts []FetchedPost
	posts, err := backend.GetPosts(ctx, user, count)
	if err != nil {
		return nil, err
	}
	log.Printf("got %d posts for user %s", len(posts), user.DisplayName)
	for _, p := range posts {
		content, err := CatString(ctx, backend, p.Content)
		if err != nil {
			return nil, err
		}
		userposts = append(userposts, FetchedPost{
			Post:             p,
			RenderedContent:  template.HTML(content),
			Author:           user.DisplayName,
			AuthorPublicName: user.PublicName,
		})
	}
	return userposts, nil
}

func sign(backend UserBackend, c *gin.Context) {
	var unr UserNameRecord
	err := c.BindJSON(&unr)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("signed unr %s", unr.PubKey)
	err = backend.PublishUser(c.Request.Context(), unr)
	if err != nil {
		errorPage(err, c)
		return
	}
	c.Status(200)
}

func acceptPost(backend Backend, c *gin.Context) {
	ctx := c.Request.Context()
	form, err := c.MultipartForm()
	if err != nil {
		errorPage(err, c)
		return
	}

	log.Printf("got post %v", form)

	poster, err := backend.GetUserById(ctx, form.Value["account"][0])
	if err != nil {
		return
	}

	images := form.File["images"]
	imagecidrs := []string{}
	for _, img := range images {
		log.Printf("found %s", img.Filename)
		f, err := img.Open()
		if err != nil {
			errorPage(err, c)
			return
		}
		cidr, err := backend.Add(ctx, f)
		if err != nil {
			errorPage(err, c)
			return
		}
		log.Printf("saved %s as %s", img.Filename, cidr)
		imagecidrs = append(imagecidrs, cidr)
	}

	posttext := form.Value["post"][0]

	cid, err := AddString(ctx, backend, posttext)
	if err != nil {
		errorPage(err, c)
		return
	}

	post := Post{
		Previous: poster.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
		Images:   imagecidrs,
	}
	postcidr, err := backend.SavePost(ctx, post)
	if err != nil {
		errorPage(err, c)
		return
	}
	poster.LastPost = postcidr
	posterrecord, err := backend.SaveUserCid(ctx, poster) //ignoring erros for now
	if err != nil {
		errorPage(err, c)
		return
	}
	c.JSON(200, posterrecord)
}

func acceptFollow(backend UserBackend, c *gin.Context) {
	ctx := c.Request.Context()
	account, faccount := c.GetPostForm("account")
	followee, ff := c.GetPostForm("followee")
	if !ff && !faccount {
		errorPage(fmt.Errorf("need account and followee"), c)
	}

	user, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
	}

	user.Follow(followee)
	followrecord, err := backend.SaveUserCid(ctx, user)
	if err != nil {
		errorPage(err, c)
	}

	c.JSON(200, followrecord)
}

func registerDisplayName(backend Backend, c *gin.Context) {
	ctx := c.Request.Context()
	account, faccount := c.GetPostForm("account")
	displayname, ff := c.GetPostForm("register")
	if !ff && !faccount {
		errorPage(fmt.Errorf("need account and followee"), c)
	}

	user, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
		return
	}

	currentaddress, err := Resolve(displayname)
	if err != nil && err != DNSNotFound {
		errorPage(err, c)
		return
	}
	if err != DNSNotFound && currentaddress != account {
		errorPage(fmt.Errorf("that dns already belongs to %s", currentaddress), c)
		return
	}

	if currentaddress == account {
		//save user cidr in case it was an eth name?go
		c.Status(http.StatusNotModified)
		return
	}

	displayname = strings.TrimSpace(displayname)
	log.Printf("regisering %s->%s", account, displayname)
	//validate valida dns host? https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names rfc 1123
	displayname, err = RegisterDNS(displayname, account)
	if err != nil {

		errorPage(err, c)
		return
	}

	user.DisplayName = displayname

	registerrecord, err := backend.SaveUserCid(ctx, user) //ignoring erros for now
	if err != nil {
		errorPage(err, c)
		return
	}
	c.JSON(200, registerrecord)
}
