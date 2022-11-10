package main

//todo put backend in a different package and shove this in north birton or seperate exe.

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"

	"github.com/araddon/dateparse"
	"github.com/gilliek/go-opml/opml"
	"github.com/mmcdole/gofeed"
)

func Import(ctx context.Context, opmplpath string) ([]string, error) {
	importedusers := []string{}
	doc, err := opml.NewOPMLFromFile(opmplpath)
	if err != nil {
		return importedusers, err
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
			userid := u.Host //was using b.GetUserId() but that doesn't make sesne need to generate public key for each?
			if seen[u.Host] {
				log.Printf("skipping %s", feed.XMLURL)
				continue
			}
			seen[u.Host] = true
			b := NewIpfsBackend(ctx, u.Host)

			author, err := b.GetUserById(userid)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if author.PublicName == "" {
				author.PublicName = u.Host
				author.DisplayName = feed.Text
			}
			importedusers = append(importedusers, userid)
			wg.Add(1)
			go func(url string) {

				log.Printf("crawling %s, %s", u.Host, url)
				Crawl(url, &author, b)
				log.Printf("saving %v", author)
				//	<-b.SaveUser(author) //not blocking yet.
				wg.Done()
			}(feed.XMLURL)
		}
	}
	wg.Wait()
	return importedusers, nil
}

func Crawl(xmlurl string, author *User, b Backend) (string, error) {
	log.Printf("fetching %s", xmlurl)
	fp := gofeed.NewParser()
	fp.UserAgent = "github.com/paulgmiller/zebu"
	feed, err := fp.ParseURL(xmlurl)
	if err != nil {
		return "", fmt.Errorf("%s fetching %s", err, xmlurl)
	}

	exisitngposts, err := b.GetPosts(*author, 10)
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
