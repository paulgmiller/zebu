package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"
	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
)

func main() {
	//https://github.com/urfave/cli/blob/master/docs/v2/manual.md#subcommands
	sMsg := flag.String("m", "", "what do you want to post?")
	keyName := flag.String("key", "zebu", "what ipns key are we using")
	followee := flag.String("follow", nobody, "add somone to your follows")
	resolve := flag.String("resolve", nobody, "look them up")
	readposts := flag.Bool("read", false, "dummp latest posts from people i follow")
	//unfollow := flag.String("unfollow", "nobody", "remove somone to your follows")
	flag.Parse()
	ctx := context.Background()

	backend := NewIpfsBackend(ctx, *keyName)

	if *followee != nobody {
		follow(backend, *followee)
		return
	}

	if *resolve != nobody {
		ResolveEns(*resolve)
		return
	}

	if *sMsg != "" {
		post(backend, *sMsg)
		return
	}

	if *readposts {
		read(backend)
		return
	}

	serve(backend)
}

//TODO unfollow
func follow(backend Backend, followee string) {
	user, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		log.Fatalf(err.Error())
	}
	user.Follows = append(user.Follows, followee)
	backend.SaveUser(user)
}

func read(backend Backend) {
	me, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		log.Fatalf(err.Error())
	}
	var followedposts []Post
	for _, follow := range me.Follows {
		f, err := backend.GetUserById(follow)
		if err != nil {
			log.Fatalf(err.Error())
		}
		posts, err := backend.GetPosts(f, 10)
		if err != nil {
			log.Fatalf("can't get posts: %s", err)
		}
		followedposts = append(followedposts, posts...)
	}
	//sort the posts by creat time?
	//add author
	for _, post := range followedposts {
		fmt.Println(post.Content)
		if !post.Created.IsZero() {
			fmt.Printf("posted at %s\n", post.Created)
		}
		content, err := backend.Cat(post.Content)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(content))

	}
}

func post(backend Backend, msg string) {
	var user User
	me, err := backend.GetUserById(backend.GetUserId())
	if err != nil {
		log.Fatalf(err.Error())
	}

	cid, err := backend.Add(msg)
	if err != nil {
		log.Fatalf("error adding content: %s", err)
	}

	post := Post{
		Previous: user.LastPost,
		Content:  cid,
		Created:  time.Now().UTC(),
	}
	err = backend.SavePost(post, me)
	if err != nil {
		log.Fatalf("Failed to publish %s", err)
	}
}
