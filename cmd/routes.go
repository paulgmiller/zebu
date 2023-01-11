package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"paulgmiller/zebu/zebu"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
)

func replaceUrlKeys(c *gin.Context) string {
	url := c.Request.URL.Path
	for _, p := range c.Params {
		url = strings.Replace(url, p.Value, ":"+p.Key, 1)
	}
	return url
}

func serve(backend zebu.Backend) {
	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}), gin.Recovery())

	router.Use(promHandler(middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})))

	//https://gin-gonic.com/docs/examples/bind-single-binary-with-template/
	t, err := loadTemplates()
	if err != nil {
		log.Fatalf("couldn't load template, %s", err)
	}
	router.SetHTMLTemplate(t)
	router.GET("/", func(c *gin.Context) {
		account, err := c.Cookie("zebu_account")
		if err == http.ErrNoCookie {
			rand(backend, c)
			return
		}
		userfeed(backend, c, account)
	})

	router.GET("/rand", func(c *gin.Context) {
		rand(backend, c)
	})

	router.POST("/post", func(c *gin.Context) {
		acceptPost(backend, c)
	})

	router.POST("/sign", func(c *gin.Context) {
		sign(backend, c)
	})

	router.GET("/healthz", func(c *gin.Context) {
		if !backend.Healthz(c.Request.Context()) {
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

	router.StaticFS("/static", loadStatic())

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Print(router.Run(":9000").Error())

}

func reader(backend zebu.Backend, c *gin.Context) (zebu.User, error) {
	account, err := c.Cookie("zebu_account")
	if err == http.ErrNoCookie {
		return zebu.User{}, nil // a little wierd
	}
	return backend.GetUserById(c.Request.Context(), account)

}

var defaultOffered = []string{"text/html", "application/json"}

//https://go.dev/blog/pipelines
//https://stackoverflow.com/questions/25142016/how-to-return-a-error-from-a-goroutine-through-channels

func mergeUsers(ctx context.Context, backend zebu.Backend, users []string, count int) <-chan zebu.FetchedPost {
	var allposts = make(chan zebu.FetchedPost)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, u := range users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			author, err := backend.GetUserById(ctx, user)
			if err != nil {
				fallback := fmt.Sprintf("error getting user %s, %s", user, err)
				log.Printf(fallback)
				allposts <- zebu.FetchedPost{RenderedContent: template.HTML(fallback), Author: user, Post: zebu.Post{}}
				return
			}

			for p := range userPosts(ctx, backend, author, count) {
				allposts <- p
			}

		}(u)
	}
	go func() {
		wg.Wait()
		close(allposts)
	}()

	return allposts
}

//really no library for this?
func ToSlice[T any](ch <-chan T) []T {
	ts := make([]T, 0)
	for t := range ch {
		ts = append(ts, t)
	}
	return ts
}

func rand(backend zebu.Backend, c *gin.Context) {
	users := backend.RandomUsers(3)
	log.Printf("getting random users %v", users)
	ctx := c.Request.Context()
	randpostchan := mergeUsers(ctx, backend, users, 3)

	randposts := ToSlice(randpostchan)
	sortposts(randposts)

	reader, err := reader(backend, c)
	if err != nil {
		errorPage(err, c)
		return
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered: defaultOffered,
		Data: gin.H{
			"Posts":     randposts,
			"Reader":    reader.Name(),
			"ReaderKey": reader.PublicKey(),
		},
		HTMLName: "feed.tmpl"})
}

//sort by create time. users could lie abotu time but trust for now
func sortposts(posts []zebu.FetchedPost) {
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Created.After(posts[j].Created)
	})
}

//show what a user is following rahter than their posts.
func userfeed(backend zebu.Backend, c *gin.Context, account string) {
	ctx := c.Request.Context()
	me, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
		return
	}
	followedpostschan := mergeUsers(ctx, backend, me.Follows, 3)

	//show them random users if they have no one to follow? nah do this on html
	followedposts := ToSlice(followedpostschan)
	sortposts(followedposts)
	name := me.DisplayName
	if name == "" {
		name = me.PublicName
	}

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered: defaultOffered,
		Data: gin.H{
			"Posts":        followedposts,
			"Reader":       me.Name(),
			"ReaderKey":    me.PublicKey(),
			"FeedOwner":    me.Name(), //allow us to see others feeds by passing this in.
			"FeedOwnerKey": me.PublicKey(),
		},
		HTMLName: "feed.tmpl"})

}

func errorPage(err error, c *gin.Context) {
	log.Printf("ERROR: %s", err.Error())
	c.JSON(500, gin.H{"msg": err.Error()})
}

type simpleuser struct {
	Id string `uri:"id" binding:"required"`
}

func userpage(backend zebu.Backend, c *gin.Context) {
	ctx := c.Request.Context()

	//todo kill this silly type
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
	account, err := zebu.Resolve(account)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("resolved to %s", account)

	followed := false

	reader, err := reader(backend, c)
	if err == nil {
		log.Printf("seeing if %s is in %v", account, reader.Follows)
		for _, f := range reader.Follows {
			//too expensive for large number of followers? need to cache Resolve.
			faccount, err := zebu.Resolve(f)
			if err == nil && faccount == account {
				followed = true
			}
		}
	}

	author, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
		return
	}

	userposts := userPosts(ctx, backend, author, 10)

	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered: defaultOffered,
		Data: gin.H{
			"Posts":     ToSlice(userposts),
			"Author":    author.Name(),
			"AuthorKey": author.PublicKey(),
			"Followed":  followed,
			"Reader":    reader.Name(),
			"ReaderKey": reader.PublicKey(),
		},
		HTMLName: "userpage.tmpl"})
}

func userPosts(ctx context.Context, backend zebu.Backend, user zebu.User, count int) <-chan zebu.FetchedPost {

	posts := backend.GetPosts(ctx, user, count)
	var wg sync.WaitGroup
	var userposts = make(chan zebu.FetchedPost, count)
	for p := range posts {
		wg.Add(1)
		go func(p zebu.Post) {
			defer wg.Done()
			content, err := zebu.CatString(ctx, backend, p.Content)
			if err != nil {
				content = fmt.Sprintf("error rendering post: %s %s", err.Error())
			}

			userposts <- zebu.FetchedPost{
				Post:            p,
				RenderedContent: template.HTML(content),
				Author:          user.Name(),
			}
		}(p)
	}
	go func() {
		wg.Wait()
		close(userposts)
	}()

	return userposts
}

func sign(backend zebu.UserBackend, c *gin.Context) {
	var unr zebu.UserNameRecord
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

func acceptPost(backend zebu.Backend, c *gin.Context) {
	ctx := c.Request.Context()
	form, err := c.MultipartForm()
	if err != nil {
		errorPage(err, c)
		return
	}

	log.Printf("got post %v", form)

	user, err := zebu.Resolve(form.Value["account"][0])
	if err != nil {
		errorPage(err, c)
		return
	}

	poster, err := backend.GetUserById(ctx, user)
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

	cid, err := zebu.AddString(ctx, backend, posttext)
	if err != nil {
		errorPage(err, c)
		return
	}

	post := zebu.Post{
		Previous: poster.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
		Images:   imagecidrs,
		Author:   poster.Name(),
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

func acceptFollow(backend zebu.UserBackend, c *gin.Context) {
	ctx := c.Request.Context()
	account, faccount := c.GetPostForm("account")
	followee, ff := c.GetPostForm("followee")
	if !ff && !faccount {
		errorPage(fmt.Errorf("need account and followee"), c)
	}
	log.Printf("got follow %s %s", account, followee)
	account, err := zebu.Resolve(account)
	if err != nil {
		errorPage(err, c)
		return
	}

	user, err := backend.GetUserById(ctx, account)
	if err != nil {
		errorPage(err, c)
	}

	//resolve folowee so we don't add garbage?
	_, err = zebu.Resolve(account)
	if err != nil {
		errorPage(err, c)
		return
	}
	user.Follow(followee)

	followrecord, err := backend.SaveUserCid(ctx, user)
	if err != nil {
		errorPage(err, c)
	}

	c.JSON(200, followrecord)
}

func registerDisplayName(backend zebu.Backend, c *gin.Context) {
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

	currentaddress, err := zebu.Resolve(displayname)
	if err != nil && err != zebu.DNSNotFound {
		errorPage(err, c)
		return
	}
	if err != zebu.DNSNotFound && currentaddress != account {
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
	displayname, err = zebu.RegisterDNS(displayname, account)
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
