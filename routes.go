package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
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
		account, err := c.Cookie("zebu_account")
		if err == http.ErrNoCookie {
			home(backend, c)
		}
		userfeed(backend, c, account)
	})
	router.POST("/post", func(c *gin.Context) {
		acceptPost(backend, c)
	})

	router.POST("/sign", func(c *gin.Context) {
		sign(backend, c)
	})

	router.POST("/follow", func(c *gin.Context) {
		acceptFollow(backend, c)
	})
	/*router.POST("/register", func(c *gin.Context) {
		registerPublicName(backend, c)
	})
	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{})
	})*/

	router.GET("/user/:id", func(c *gin.Context) {
		userpage(backend, c)
	})
	router.GET("/img/:cidr", func(c *gin.Context) {
		cidr := c.Param("cidr")
		imgreader, err := backend.CatReader(cidr)
		if err != nil {
			errorPage(err, c)
			return
		}
		//can't get lenth without buf.
		buf := &bytes.Buffer{}
		buf.ReadFrom(imgreader)

		c.DataFromReader(http.StatusOK, int64(buf.Len()), "image/*", buf, map[string]string{})
	})

	log.Print(router.Run(":9000").Error())
}

func home(backend Backend, c *gin.Context) {
	//
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts":          []FetchedPost{},
		"UserId":         c.Cookie,
		"UserPublicName": "nobody",
	})
}

//show what a user is following rahter than their posts.
func userfeed(backend Backend, c *gin.Context, account string) {
	me, err := backend.GetUserById(account)
	if err != nil {
		errorPage(err, c)
		return
	}

	/*if me.PublicName == "" {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{})
		return
	}*/

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
				RenderedContent:  template.HTML(content),
				Author:           follow,
				AuthorPublicName: f.PublicName,
			})
		}
	}
	mine, err := backend.GetPosts(me, 1)
	if err != nil {
		errorPage(err, c)
		return
	}
	if len(mine) > 0 {
		p := mine[0]
		content, err := backend.Cat(p.Content)
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

	//users could lie abotu time but trust for now
	sort.Slice(followedposts, func(i, j int) bool { return followedposts[i].Created.After(followedposts[j].Created) })
	c.HTML(http.StatusOK, "user.tmpl", gin.H{
		"Posts":          followedposts,
		"UserId":         account,
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

	if c.Query("raw") == "true" {
		c.IndentedJSON(http.StatusOK, user)
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
			RenderedContent:  template.HTML(content),
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

func sign(backend UserBackend, c *gin.Context) {
	var unr UserNameRecord
	err := c.BindJSON(&unr)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("signed unr %s", unr.PubKey)
	err = backend.PublishUser(unr)
	if err != nil {
		errorPage(err, c)
		return
	}
	c.Status(200)
}

func acceptPost(backend Backend, c *gin.Context) {

	form, err := c.MultipartForm()
	if err != nil {
		errorPage(err, c)
		return
	}

	log.Printf("got post %v", form)

	poster, err := backend.GetUserById(form.Value["account"][0])
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
		cidr, err := backend.Add(f)
		if err != nil {
			errorPage(err, c)
			return
		}
		log.Printf("saved %s as %s", img.Filename, cidr)
		imagecidrs = append(imagecidrs, cidr)
	}

	posttext := form.Value["post"][0]

	cid, err := AddString(backend, posttext)
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
	postcidr, err := backend.SavePost(post)
	if err != nil {
		errorPage(err, c)
		return
	}
	poster.LastPost = postcidr
	posterrecord, err := backend.SaveUserCid(poster) //ignoring erros for now
	if err != nil {
		errorPage(err, c)
		return
	}
	jsonrecord, _ := json.Marshal(posterrecord)
	log.Printf("returning %s", jsonrecord)
	c.JSON(200, posterrecord)
}

func acceptFollow(backend UserBackend, c *gin.Context) {
	account, faccount := c.GetPostForm("account")
	followee, ff := c.GetPostForm("followee")
	if !ff && !faccount {
		errorPage(fmt.Errorf("need account and followee"), c)
	}

	user, err := backend.GetUserById(account)
	if err != nil {
		errorPage(err, c)
	}

	user.Follow(followee)
	followrecord, err := backend.SaveUserCid(user)
	if err != nil {
		errorPage(err, c)
	}

	jsonrecord, _ := json.Marshal(followrecord)
	log.Printf("returning %s", jsonrecord)
	c.JSON(200, followrecord)
}

type simpleRegister struct {
	PublicName string `form:"publicname"`
}

/* This still makes some sense. Give them a domain name if their account has more than some eth in it.
func registerPublicName(backend Backend, c *gin.Context) {
	var simpleRegister simpleRegister
	c.Bind(&simpleRegister)
	publicname := strings.TrimSpace(simpleRegister.PublicName)
	url := "https://registrar.northbriton.net/reserve/" + publicname
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(backend.GetUserId()))
	if err != nil {
		errorPage(err, c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		errorPage(err, c)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		errorPage(fmt.Errorf("got %d : %s", resp.StatusCode, string(body)), c)
	}

	user, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		errorPage(err, c)
	}
	user.PublicName = publicname + ".northbriton.net"
	backend.SaveUserCid(user)

	c.Redirect(http.StatusFound, "/")
}
*/
