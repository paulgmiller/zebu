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

	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
	ipfs "github.com/ipfs/go-ipfs-api"
)

type Post struct {
	Previous string
	Content  string
}

func main() {

	ipfsShell := ipfs.NewShell("localhost:5001")

	sMsg := flag.String("m", "", "what do you want to post?")
	keyName := flag.String("key", "zebu", "what ipns key are we using")

	flag.Parse()
	ctx := context.Background()

	keys, err := ipfsShell.KeyList(ctx)
	if err != nil {
		log.Fatalf("Can't get keys %s", err)
	}
	var key *ipfs.Key
	for _, k := range keys {
		//fmt.Printf("Got key %s\n", k.Name)
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

func read(ipfsShell *ipfs.Shell, key *ipfs.Key) {
	head, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		log.Fatalf("can't resolve key %s. Maybe post something %s", key.Name, err)
	}
	for head != "" {
		reader, err := ipfsShell.Cat(head)
		if err != nil {
			log.Fatalf("can't resolve key %s. Maybe post something %s", key.Name, err)
		}
		defer reader.Close()
		dec := json.NewDecoder(reader)
		var post Post
		err = dec.Decode(&post)
		if err != nil {
			log.Fatalf("couldn't decode post %v", err)
		}
		fmt.Println(post.Content)
		contentreader, err := ipfsShell.Cat(post.Content)
		defer reader.Close()
		content, err := ioutil.ReadAll(contentreader)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(string(content))

		head = post.Previous
	}
}

func post(ipfsShell *ipfs.Shell, key *ipfs.Key, msg string) {
	current, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			current = ""
		} else {
			log.Fatalf("can't resolve key: %s", err)
		}
	}

	cid, err := ipfsShell.Add(strings.NewReader(msg))
	if err != nil {
		log.Fatalf("error adding content: %s", err)
	}

	post := Post{
		Previous: current,
		Content:  cid,
	}
	jpost, _ := json.Marshal(post)
	postcid, err := ipfsShell.Add(bytes.NewReader(jpost))
	if err != nil {
		log.Fatalf("error adding post: %s", err)
	}
	fmt.Printf("%s added %s as post %s", msg, cid, postcid)
	resp, err := ipfsShell.PublishWithDetails(postcid, key.Name, 0, 0, false)
	if err != nil {
		log.Fatalf("Failed to publish %s", err)
	}
	fmt.Printf("Posted %+v\n", resp)
}
