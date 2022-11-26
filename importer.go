package main

//todo put backend in a different package and shove this in north birton or seperate exe.

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/araddon/dateparse"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gilliek/go-opml/opml"
	"github.com/mmcdole/gofeed"
)

const importskeypath = "import_keys"

func Import(ctx context.Context, opmplpath string) ([]string, error) {
	importedusers := []string{}
	doc, err := opml.NewOPMLFromFile(opmplpath)
	if err != nil {
		return importedusers, err
	}
	b := NewIpfsBackend(ctx)

	if _, err := os.Stat(importskeypath); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(importskeypath, os.ModePerm); err != nil {
			return nil, err
		}
	}
	var wg sync.WaitGroup
	seen := map[string]bool{}
	for _, o := range doc.Body.Outlines {
		for _, feed := range o.Outlines {
			u, err := url.Parse(feed.XMLURL)
			if err != nil {
				log.Printf("can't parse %s", feed.XMLURL)
				continue
			}
			trimurl := u.Host + "/" + u.Path
			urlhash := hex.EncodeToString(sha256.New().Sum([]byte(trimurl)))
			log.Printf("%s->%s", trimurl, urlhash)
			if seen[urlhash] {
				log.Printf("skipping %s", feed.XMLURL)
				continue
			}
			keyfile := importskeypath + "/" + urlhash //these are secerts where should we save them?
			privatekey, err := crypto.LoadECDSA(keyfile)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Printf("load of key failed %s", err)
					continue
				}
				privatekey, err = crypto.GenerateKey()
				if err != nil {
					log.Println(err.Error())
					continue
				}
				if err := crypto.SaveECDSA(keyfile, privatekey); err != nil {
					log.Printf("save of key failed %s", err)
					continue
				}
			}

			addr := crypto.PubkeyToAddress(privatekey.PublicKey).Hex()
			author, err := b.GetUserById(addr)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if author.PublicName == "" {
				//need to generate a public key or use the node public key.
				author.PublicName = addr
			}

			if author.DisplayName == "" {

				//resolve dns?
				dp, err := RegisterDNS(simplifyTitle(feed.Title), addr)
				if err != nil {
					log.Println(err.Error())
					continue
				}
				author.DisplayName = dp
			}
			author.ImportSource = trimurl
			importedusers = append(importedusers, author.DisplayName)
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				log.Printf("crawling %s, %s", u.Host, url)
				post, err := Crawl(url, author, b)
				if err != nil {
					log.Println(err.Error())
					return
				}
				author.LastPost = post
				err = publishWithKey(author, b, privatekey)
				if err != nil {
					log.Println(err.Error())
					return
				}
			}(feed.XMLURL)
		}
	}
	wg.Wait()
	return importedusers, nil
}

var alphanum = "abcdefghijklmnopqrstuvwxyz1234567890"

func simplifyTitle(title string) string {
	var b strings.Builder
	title = strings.ToLower(title)
	for _, ch := range title {
		if strings.Contains(alphanum, string(ch)) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func publishWithKey(author User, b UserBackend, privatekey *ecdsa.PrivateKey) error {
	unr, err := b.SaveUserCid(author) //not blocking yet.
	if err != nil {
		return fmt.Errorf("could not save %v, %w", author, err)
	}

	if err := unr.Sign(privatekey); err != nil {
		return err
	}

	if !unr.Validate() {
		return fmt.Errorf("coul not validate  %v", unr)
	}
	err = b.PublishUser(unr)
	if err != nil {
		return fmt.Errorf("couln't publish %v, %w", unr, err)
	}
	return nil
}

func Crawl(xmlurl string, author User, b Backend) (string, error) {
	log.Printf("fetching %s", xmlurl)
	fp := gofeed.NewParser()
	fp.UserAgent = "github.com/paulgmiller/zebu"
	feed, err := fp.ParseURL(xmlurl)
	if err != nil {
		return "", fmt.Errorf("%s fetching %s", err, xmlurl)
	}

	exisitngposts, err := b.GetPosts(author, 10)
	if err != nil {
		return "", fmt.Errorf("%s parsing %s", err, xmlurl)
	}

	oldposts := map[string]Post{}
	for _, p := range exisitngposts {
		oldposts[p.Content] = p
	}

	previous := ""
	if len(feed.Items) == 0 {
		log.Printf("Got 0 items from %s", xmlurl)
	}
	for i := len(feed.Items) - 1; i >= 0; i-- {
		item := feed.Items[i]

		time, err := dateparse.ParseAny(item.Published)
		if err != nil {
			time, err = dateparse.ParseAny(item.Updated)
			if err != nil {
				fmt.Println(err.Error())
			}
		}

		cid, err := AddString(b, item.Title+"<br/>"+item.Description)
		if err != nil {
			log.Printf("error adding content: %s", err)
			return "", err
		}
		var post Post
		if oldpost, found := oldposts[cid]; found {
			post = oldpost //stop doing this once we have one that doesn't match so an edit doesn't delete things?
		} else {
			post = Post{
				Previous: previous,
				Content:  cid,
				Created:  time,
			}
		}
		previous, err = b.SavePost(post)
		if err != nil {
			log.Printf("error saving post: %s", err)
			return "", err
		}
	}

	return previous, nil
}
