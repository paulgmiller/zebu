package test

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"testing"
	"time"
)

const account = "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"

//zebu needs to move out of main package.
type Post struct {
	Previous string
	Content  string
	Images   []string  //this makes it hard to do images inline? don't care?
	Created  time.Time //can't actually trust this
}

type FetchedPost struct {
	Post
	RenderedContent  template.HTML
	Author           string
	AuthorPublicName string
}

type PageResult struct {
	Posts          []FetchedPost
	UserId         string
	UserPublicName string
}

func endpoint() string {
	//allow overrideing with env var
	return "http://localhost:9000"
}

func must(err error, t *testing.T, msg string) {
	if err != nil {
		t.Fatalf("%s:%s", msg, err.Error())
	}
}

func TestRand(t *testing.T) {

	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint()+"/rand", nil)
	must(err, t, "req")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	must(err, t, "do")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	must(err, t, "read")
	fmt.Println(string(body))
	var result PageResult
	err = json.Unmarshal(body, &result)
	//err = json.NewDecoder(resp.Body).Decode(resp.Body)
	must(err, t, "decode")
	if len(result.Posts) < 3 {
		t.Fatalf("too few posts %d", len(result.Posts))
	}
	authors := map[string]bool{}
	for _, p := range result.Posts {
		authors[p.Author] = true
	}
	fmt.Printf("authors: %v", authors)
	if len(authors) < 3 {
		t.Fatalf("not enough  authors %d", len(authors))
	}

}

func TestUserAccount(t *testing.T) {

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint()+"/user/"+account, nil)
	must(err, t, "req")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	must(err, t, "do")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	must(err, t, "read")
	fmt.Println(string(body))
	var result PageResult
	err = json.Unmarshal(body, &result)
	//err = json.NewDecoder(resp.Body).Decode(resp.Body)
	must(err, t, "decode")
	if len(result.Posts) < 3 {
		t.Fatalf("too few posts %d", len(result.Posts))
	}
}
