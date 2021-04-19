package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-dnslink"
	ipfs "github.com/ipfs/go-ipfs-api"
)

func main() {
	ipfsShell := ipfs.NewShell("localhost:5001")
	//https://github.com/urfave/cli/blob/master/docs/v2/manual.md#subcommands
	sMsg := flag.String("m", "", "what do you want to post?")
	keyName := flag.String("key", "zebu", "what ipns key are we using")
	followee := flag.String("follow", nobody, "add somone to your follows")
	resolve := flag.String("resolve", nobody, "look them up")
	serve := flag.Bool("serve", false, "serve up web ui")
	//unfollow := flag.String("unfollow", "nobody", "remove somone to your follows")
	flag.Parse()
	ctx := context.Background()

	keys, err := ipfsShell.KeyList(ctx)
	if err != nil {
		log.Fatalf("Can't get keys %s", err)
	}
	var key *ipfs.Key
	for _, k := range keys {
		if k.Name == *keyName {
			key = k
		}
	}
	if key == nil {
		key, err = ipfsShell.KeyGen(ctx, *keyName)
		if err != nil {
			log.Fatalf("Can't create keys %s", *keyName)
		}
	}

	if *serve {
		router := gin.Default()
		router.LoadHTMLFiles("index.tmpl")
		router.GET("/", func(c *gin.Context) {
			userfeed(ipfsShell, key, c)
		})
		router.POST("/post", func(c *gin.Context) {
			acceptPost(ipfsShell, key, c)
		})
		router.GET("/user/:id", func(c *gin.Context) {
			userpage(ipfsShell, c)
		})
		log.Print(router.Run(":9000").Error())
		return
	}

	if *followee != nobody {
		follow(ipfsShell, key, *followee)
		return
	}

	if *resolve != nobody {
		//usercid, err := ipfsShell.Resolve(*resolve)
		link, err := dnslink.Resolve(*resolve)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println(link)
		}

		return
	}

	if *sMsg == "" {
		read(ipfsShell, key)
		return
	}
	post(ipfsShell, key, *sMsg)
}

func readJson(ipfsShell *ipfs.Shell, cid string, obj interface{}) error {
	reader, err := ipfsShell.Cat(cid)
	if err != nil {
		return err
	}
	defer reader.Close()
	dec := json.NewDecoder(reader)
	return dec.Decode(obj)
}

func writeJson(ipfsShell *ipfs.Shell, obj interface{}) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	err := enc.Encode(obj)
	if err != nil {
		return "", err
	}
	return ipfsShell.Add(&b)
}

//TODO unfollow
func follow(ipfsShell *ipfs.Shell, key *ipfs.Key, followee string) {
	var user User
	usercid, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			user = User{}
		} else {
			log.Fatalf("can't resolve key: %s", err)
		}
	} else {
		err = readJson(ipfsShell, usercid, &user)
		if err != nil {
			log.Fatalf("error getting user : %s", err)
		}
	}
	user.Follows = append(user.Follows, followee)
	usercid, err = writeJson(ipfsShell, &user)
	if err != nil {
		log.Fatalf("error updating user: %s", err)
	}

	resp, err := ipfsShell.PublishWithDetails(usercid, key.Name, 0, 0, false)
	if err != nil {
		log.Fatalf("Failed to publish %s", err)
	}
	fmt.Printf("Following %s\n", user.Follows)
	fmt.Printf("Posted user %s to %s\n", usercid, resp.Name)

}

func getPosts(ipfsShell *ipfs.Shell, user User, count int) ([]Post, error) {
	head := user.LastPost
	var posts []Post
	for i := 0; head != "" && i < count; i++ {
		var post Post
		if err := readJson(ipfsShell, head, &post); err != nil {
			return posts, fmt.Errorf("can't resolve content %s: %w", head, err)
		}
		posts = append(posts, post)
		head = post.Previous
	}
	return posts, nil
}

const ipnsprefix = "/ipns/"

func getUser(ipfsShell *ipfs.Shell, userlookup string) (User, error) {
	link, err := dnslink.Resolve(userlookup)
	if err != nil && strings.HasPrefix(link, ipnsprefix) {
		userlookup = link[len(ipnsprefix):]
	}

	usercid, err := ipfsShell.Resolve(userlookup)
	if err != nil {
		return User{}, err
	}
	var user User
	if err := readJson(ipfsShell, usercid, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func read(ipfsShell *ipfs.Shell, key *ipfs.Key) {

	me, err := getUser(ipfsShell, key.Id)
	if err != nil {
		log.Fatalf("can't get user %s: %s", key.Id, err)
	}
	var followedposts []Post
	for _, follow := range me.Follows {
		f, err := getUser(ipfsShell, follow)
		if err != nil {
			log.Fatalf("can't get user %s: %s", key.Id, err)
		}
		posts, err := getPosts(ipfsShell, f, 10)
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
		contentreader, err := ipfsShell.Cat(post.Content)
		if err != nil {
			//just continue?
			log.Fatalf("can't get content %s: %s", post.Content, err)
		}
		defer contentreader.Close()
		content, err := ioutil.ReadAll(contentreader)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(content))

	}
}

func post(ipfsShell *ipfs.Shell, key *ipfs.Key, msg string) {
	var user User
	usercid, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			user = User{}
		} else {
			log.Fatalf("can't resolve key: %s", err)
		}
	} else {
		err = readJson(ipfsShell, usercid, &user)
		if err != nil {
			log.Fatalf("error getting user : %s", err)
		}
	}

	cid, err := ipfsShell.Add(strings.NewReader(msg))
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
		log.Fatalf("error adding post: %s", err)
	}
	fmt.Printf("%s added %s as post %s\n", msg, cid, postcid)
	user.LastPost = postcid
	usercid, err = writeJson(ipfsShell, &user)
	if err != nil {
		log.Fatalf("error updating user: %s", err)
	}

	resp, err := ipfsShell.PublishWithDetails(usercid, key.Name, 0, 0, false)
	if err != nil {
		log.Fatalf("Failed to publish %s", err)
	}

	fmt.Printf("Posted user %s to %s\n", usercid, resp.Name)
}
