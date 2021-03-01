package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
	ipfs "github.com/ipfs/go-ipfs-api"
)

type Post struct {
	Previous string
	Content  string
}

func main() {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	ipfsShell := ipfs.NewShell("localhost:5001")

	sMsg := flag.String("m", "", "what do you want to post?")
	keyName := flag.String("key", "zebu", "what ipns key are we using")
	flag.Parse()
	if *sMsg == "" {
		logger.Fatal("have to give me something\n")

	}
	ctx := context.Background()
	keys, err := ipfsShell.KeyList(ctx)
	if err != nil {
		logger.Fatalf("Can't get keys %s", err)
	}
	var key *ipfs.Key
	for _, k := range keys {
		if k.Name == *keyName {
			key = key
		}
	}
	if key == nil {
		key, err = ipfsShell.KeyGen(ctx, *keyName)
		if err != nil {
			logger.Fatalf("Can't crate  keys %s", keyName)
		}
	}

	current, err := ipfsShell.Resolve(key.Id)
	if err != nil {
		logger.Fatalf("can't resolve key: %s", err)
	}

	cid, err := ipfsShell.Add(strings.NewReader(*sMsg))
	if err != nil {
		logger.Fatalf("error adding content: %s", err)
	}
	fmt.Printf("%s added %s", *sMsg, cid)
	post := Post{
		Previous: current,
		Content:  cid,
	}
	jpost, _ := json.Marshal(post)
	postcid, err := ipfsShell.Add(bytes.NewReader(jpost))
	if err != nil {
		logger.Fatalf("error adding post: %s", err)
	}
	fmt.Printf("%s added %s", jpost, postcid)
	resp, err := ipfsShell.PublishWithDetails(postcid, key.Name, 0, 90, false)
	if err != nil {
		logger.Fatalf("Failed to publish %s", err)
	}
	fmt.Printf("Posted %+v", resp)

}
