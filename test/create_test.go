package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"paulgmiller/zebu/zebu"
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

}
