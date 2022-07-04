package main

//todo put backend in a different package and shove this in north birton or seperate exe.

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gilliek/go-opml/opml"
	"github.com/ungerik/go-rss"
)

func Import(ctx context.Context, opmplpath string) ([]string, error) {
	importedusers := []string{}
	doc, err := opml.NewOPMLFromFile(opmplpath)
	if err != nil {
		return importedusers, err
	}

	for _, o := range doc.Body.Outlines {
		for _, feed := range o.Outlines {
			u, err := url.Parse(feed.XMLURL)
			if err != nil {
				log.Printf("can't parse %s", feed.XMLURL)
				continue
			}
			b := NewIpfsBackend(ctx, u.Host)

			author, err := b.GetUserById(b.GetUserId())
			if err != nil {
				log.Printf(err.Error())
				continue
			}
			if author.PublicName == "" {
				author.PublicName = u.Host
				author.DisplayName = feed.Text
			}

			log.Printf("crawling %s, %s", u.Host, feed.XMLURL)
			Crawl(feed.XMLURL, &author, b)
			importedusers = append(importedusers, b.GetUserId())
			log.Printf("saving %v", author)
			<-b.SaveUser(author) //block
		}
	}
	return importedusers, nil
}

func Crawl(xmlurl string, author *User, b Backend) (string, error) {
	log.Printf("fetching %s", xmlurl)
	resp, err := http.Get(xmlurl)
	if err != nil {
		return "", fmt.Errorf("%s fetching %s", err, xmlurl)

	}
	channel, err := rss.Regular(resp)
	if err != nil {
		return "", fmt.Errorf("%s parsing %s", err, xmlurl)
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

	log.Printf("Got %d items", len(channel.Item))
	for i := len(channel.Item) - 1; i >= 0; i-- {
		item := channel.Item[i]

		time, err := item.PubDate.Parse()
		if err != nil {
			fmt.Println(err)
		}

		cid, err := AddString(b, item.Title+"<br/>"+item.Description)
		if err != nil {
			log.Printf("error adding content: %s", err)
			return "", err
		}
		//todo see if we already have a post with this cidr so we don't overwrite
		post := Post{
			Previous: previous,
			Content:  cid,
			Created:  time,
		}
		previous, err = b.SavePost(post)
		if err != nil {
			log.Printf("error saving post: %s", err)
			return "", err
		}
	}
	return previous, nil
}
