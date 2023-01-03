package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"paulgmiller/zebu/zebu"
	"testing"
)

const account = "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"

type UserResult struct {
	Posts  []zebu.FetchedPost
	Author string
	Reader string
}

func endpoint() string {
	//allow overrideing with env var
	if e, ok := os.LookupEnv("ZEBU_ENDPOINT"); ok {
		return e
	}
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
	var result UserResult
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

	if result.Author != "" {
		t.Fatalf("author should be empty")
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
	var result UserResult
	err = json.Unmarshal(body, &result)
	//err = json.NewDecoder(resp.Body).Decode(resp.Body)
	must(err, t, "decode")
	if len(result.Posts) < 3 {
		t.Fatalf("too few posts %d", len(result.Posts))
	}

	if result.Author != "johnwilkes.northbriton.net" {
		t.Fatalf("author should be set")
	}
}
