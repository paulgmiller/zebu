package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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

	router.GET("/sign", func(c *gin.Context) {
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

//show what a user is following rahter than their posts.
func userfeed(backend Backend, c *gin.Context) {
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

	me, err := backend.GetUserById(simpleUser.Id)
	if err != nil {
		errorPage(err, c)
		return
	}

	if me.PublicName == "" {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{})
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
				RenderedContent:  template.HTML(content),
				Author:           follow,
				AuthorPublicName: f.PublicName,
			})
		}
	}
	//users could lie abotu time but trust for now
	sort.Slice(followedposts, func(i, j int) bool { return followedposts[i].Created.After(followedposts[j].Created) })
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts":          followedposts,
		"UserId":         simpleUser.Id,
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

type simplePost struct {
	Post string `form:"post"`
}

func sign(backend Backend, c *gin.Context) {
	c.HTML(http.StatusOK, "sign.tmpl", gin.H{})
}

func acceptPost(backend Backend, c *gin.Context) {

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

	me, err := backend.GetUserById(simpleUser.Id)
	if err != nil {
		errorPage(err, c)
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		errorPage(err, c)
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

	var simplePost simplePost
	c.Bind(&simplePost)
	if simplePost.Post == "" {
		log.Printf("form: %v", c.Request.Form)
		errorPage(fmt.Errorf("Empty post"), c)
		return
	}

	cid, err := AddString(backend, simplePost.Post)
	if err != nil {
		errorPage(err, c)
		return
	}

	post := Post{
		Previous: me.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
		Images:   imagecidrs,
	}
	postcidr, err := backend.SavePost(post)
	if err != nil {
		errorPage(err, c)
		return
	}
	me.LastPost = postcidr
	backend.SaveUserCid(me) //ignoring erros for now

	//redirect to a signing page. Eventually ajax
	c.Redirect(http.StatusFound, "/user/"+simpleUser.Id)
}

type simpleFollow struct {
	Followee string `form:"followee"` //ipns?
}

func acceptFollow(backend Backend, c *gin.Context) {
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
	}

	var simpleFollow simpleFollow
	c.Bind(&simpleFollow)
	_, err = backend.GetUserById(simpleFollow.Followee)
	if err != nil {
		errorPage(errors.Wrap(err, "couldn't resolve followee"), c)
	}

	user.Follow(simpleFollow.Followee)
	backend.SaveUserCid(user)

	//redirect to a signing page. Eventually ajax
	c.Redirect(http.StatusFound, "/user/"+simpleFollow.Followee)
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
