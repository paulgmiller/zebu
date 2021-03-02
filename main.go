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
	ipfs "github.com/ipfs/go-ipfs-api"
)

type User struct {
	LastPost string
	Follows  []string
}

type Post struct {
	Previous string
	Content  string
	Created  time.Time //can't actually trust this
}

const nobody = "nobody"

func main() {
	ipfsShell := ipfs.NewShell("localhost:5001")

	sMsg := flag.String("m", "", "what do you want to post?")
	keyName := flag.String("key", "zebu", "what ipns key are we using")
	//follow := flag.String("follow", nobody, "add somone to your follows")
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

	if *sMsg == "" {
		read(ipfsShell, key)
		return
	}
	post(ipfsShell, key, *sMsg)
}

func readJson(ipfsShell *ipfs.Shell, cid string, obj interface{}) error {
	reader, err := ipfsShell.Cat(cid)
	defer reader.Close()
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

func read(ipfsShell *ipfs.Shell, key *ipfs.Key) {
	head, err := ipfsShell.Resolve(key.Id)

	var user User
	usercid, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		log.Fatalf("can't resolve key: %s", err)
	}

	err = readJson(ipfsShell, usercid, &user)
	if err != nil {
		log.Fatalf("error getting user : %s", err)
	}
	head = user.LastPost

	if err != nil {
		log.Fatalf("can't resolve key %s. Maybe post something %s", key.Name, err)
	}
	for head != "" {
		var post Post
		err := readJson(ipfsShell, head, &post)
		if err != nil {
			log.Fatalf("can't resolve key %s. Maybe post something %s", key.Name, err)
		}
		fmt.Println(post.Content)
		if !post.Created.IsZero() {
			fmt.Printf("posted at %s\n", post.Created)
		}
		contentreader, err := ipfsShell.Cat(post.Content)
		if err != nil {
			log.Fatalf("can't get content %s: %s", post.Content, err)
		}
		defer contentreader.Close()
		content, err := ioutil.ReadAll(contentreader)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(content))

		head = post.Previous
	}
}

func post(ipfsShell *ipfs.Shell, key *ipfs.Key, msg string) {
	var user User
	usercid, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			user = User{
				LastPost: "",
				Follows:  []string{},
			}
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
