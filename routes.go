package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ipfs "github.com/ipfs/go-ipfs-api"
)

func userfeed(ipfsShell *ipfs.Shell, key *ipfs.Key, c *gin.Context) {
	me, err := getUser(ipfsShell, key.Id)
	if err != nil {
		errorPage(err, c)
		return
	}
	var followedposts []FetchedPost
	for _, follow := range me.Follows {
		f, err := getUser(ipfsShell, follow)
		if err != nil {
			errorPage(err, c)
			return
		}
		posts, err := getPosts(ipfsShell, f, 10)
		if err != nil {
			errorPage(err, c)
			return
		}
		for _, p := range posts {
			contentreader, err := ipfsShell.Cat(p.Content)
			if err != nil {
				errorPage(err, c)
				return
			}
			defer contentreader.Close()
			content, err := ioutil.ReadAll(contentreader)
			if err != nil {
				fmt.Println(err.Error())
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
		"Me":    key.Id,
	})
}

func errorPage(err error, c *gin.Context) {
	c.JSON(400, gin.H{"msg": err})
}

type simpleuser struct {
	Id string `uri:"id" binding:"required"`
}

func userpage(ipfsShell *ipfs.Shell, c *gin.Context) {
	var simpleUser simpleuser
	if err := c.ShouldBindUri(&simpleUser); err != nil {
		log.Print("Argh")
		errorPage(err, c)
		return
	}

	if simpleUser.Id == "" {
		log.Printf("failure! %s", simpleUser.Id)
		errorPage(fmt.Errorf("no user supplied"), c)
		return
	}
	log.Printf("getting user %s", simpleUser.Id)
	user, err := getUser(ipfsShell, simpleUser.Id)
	if err != nil {
		errorPage(err, c)
		return
	}
	var followedposts []FetchedPost
	posts, err := getPosts(ipfsShell, user, 10)
	if err != nil {
		errorPage(err, c)
		return
	}
	for _, p := range posts {
		contentreader, err := ipfsShell.Cat(p.Content)
		if err != nil {
			errorPage(err, c)
			return
		}
		defer contentreader.Close()
		content, err := ioutil.ReadAll(contentreader)
		if err != nil {
			fmt.Println(err.Error())
		}
		followedposts = append(followedposts, FetchedPost{
			Post:            p,
			RenderedContent: string(content),
			Author:          simpleUser.Id,
		})
	}
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"Posts": followedposts,
		"Me":    simpleUser.Id, //a little wierd
	})
}

func acceptPost(ipfsShell *ipfs.Shell, key *ipfs.Key, c *gin.Context) {
	var user User
	usercid, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			user = User{}
		} else {
			errorPage(err, c)
			return
		}
	} else {
		err = readJson(ipfsShell, usercid, &user)
		if err != nil {
			errorPage(err, c)
			return
		}
	}

	var simplePost struct {
		Post string `form:"post"`
	}
	c.Bind(&simplePost)
	cid, err := ipfsShell.Add(strings.NewReader(simplePost.Post))
	if err != nil {
		log.Fatalf("error adding content: %s", err)
	}

	post := Post{
		Previous: user.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
	}
	postcid, err := writeJson(ipfsShell, &post)
	if err != nil {
		errorPage(err, c)
		return
	}
	user.LastPost = postcid
	usercid, err = writeJson(ipfsShell, &user)
	if err != nil {
		errorPage(err, c)
		return
	}

	//is this too slow?
	resp, err := ipfsShell.PublishWithDetails(usercid, key.Name, 0, 0, false)
	if err != nil {
		errorPage(err, c)
		return
	}
	log.Printf("Posted user %s to %s\n", usercid, resp.Name)

	c.Redirect(http.StatusTemporaryRedirect, "/user/"+key.Id)
}
