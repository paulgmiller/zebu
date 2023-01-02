package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"paulgmiller/zebu/zebu"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestPost(t *testing.T) {

	ctx := context.Background()

	privatekey, err := crypto.GenerateKey()
	must(err, t, "generate key")

	addr := crypto.PubkeyToAddress(privatekey.PublicKey).Hex()

	//io.Pipe() is another option
	buf := &bytes.Buffer{}
	formwriter := multipart.NewWriter(buf)
	formwriter.WriteField("account", addr)
	formwriter.WriteField("post", "hello world")
	formwriter.Close()

	//todo: add image
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint()+"/post", io.NopCloser(buf))
	must(err, t, "req")
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+formwriter.Boundary())

	resp, err := http.DefaultClient.Do(req)
	must(err, t, "do")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	must(err, t, "read")
	fmt.Println(string(body))
	var unr zebu.UserNameRecord
	err = json.Unmarshal(body, &unr)
	//err = json.NewDecoder(resp.Body).Decode(resp.Body)
	must(err, t, "decode")
	if unr.PubKey != addr {
		t.Fatalf("bad pub key %s", unr.PubKey)
	}
	unr.Sign(privatekey)
	jbytes, err := json.Marshal(unr)
	must(err, t, "marshal")
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint()+"/sign", bytes.NewReader(jbytes))
	must(err, t, "req sign")
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	must(err, t, "do sign")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}

	//follow
	followbody := &url.Values{}
	followbody.Set("account", addr)
	followbody.Set("followee", "johnwilkes.northbriton.net")
	//todo: add image
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint()+"/follow", strings.NewReader(followbody.Encode()))
	must(err, t, "req")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = http.DefaultClient.Do(req)
	must(err, t, "do")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}
	body, err = io.ReadAll(resp.Body)
	must(err, t, "read")
	fmt.Println(string(body))

	var unr2 zebu.UserNameRecord
	err = json.Unmarshal(body, &unr2)
	//err = json.NewDecoder(resp.Body).Decode(resp.Body)
	must(err, t, "decode")
	if unr.PubKey != addr {
		t.Fatalf("bad pub key %s", unr2.PubKey)
	}
	unr2.Sign(privatekey)
	if !unr2.Validate() {
		t.Fatalf("bad signature, %s", unr2.Signature)
	}
	jbytes, err = json.Marshal(unr2)
	must(err, t, "marshal")
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint()+"/sign", bytes.NewReader(jbytes))
	must(err, t, "req sign")
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	must(err, t, "do sign")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bad status %d", resp.StatusCode)
	}

}
