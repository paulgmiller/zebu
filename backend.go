package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/ipfs/go-dnslink"
	ipfs "github.com/ipfs/go-ipfs-api"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/mitchellh/go-homedir"
)

type Backend interface {
	ContentBackend
	KeyBackend
}

type KeyBackend interface {
	ExportKey() ([]byte, error)
	EnsureKey(context.Context, string) (*ipfs.Key, error)
}

type ContentBackend interface {
	GetUserById(usercid string) (User, error)
	GetUserId() string
	SaveUser(user User) error
	GetPosts(user User, count int) ([]Post, error)
	SavePost(post Post, user User) error
	//too low level?
	Cat(cid string) (string, error) //remove with helper method.
	CatReader(cid string) (io.ReadCloser, error)
	Add(r io.Reader) (string, error)
}

type IpfsBackend struct {
	shell     *ipfs.Shell
	key       *ipfs.Key
	namecache map[string]string
	keystore  keystore.Keystore
}

func NewIpfsBackend(ctx context.Context, keyName string) *IpfsBackend {

	shell := ipfs.NewShell("localhost:5001")
	if !shell.IsUp() {
		log.Fatal("Ipfs not fond on localhost:5001 please install https://docs.ipfs.io/install/command-line/#official-distributions")
	}

	keystoredir, _ := homedir.Expand("~/.ipfs/keystore")
	if ipfspath, found := os.LookupEnv("IPFS_PATH"); found {
		keystoredir = ipfspath + "/keystore"
	}
	if _, err := os.Stat(keystoredir); os.IsNotExist(err) {
		//stupid snap
		keystoredir, _ = homedir.Expand("~/snap/ipfs/common/keystore")
	}

	ks, err := keystore.NewFSKeystore(keystoredir)
	if err != nil {
		log.Fatalf("Can't create keystore %s", keystoredir)
	}

	backend := &IpfsBackend{
		shell:     shell,
		namecache: map[string]string{},
		keystore:  ks,
	}

	if backend.key, err = backend.EnsureKey(ctx, keyName); err != nil {
		log.Fatal(err.Error())
	}

	if found, _ := ks.Has(keyName); !found {
		log.Fatal("Coudn't find key in keystore")
	}

	return backend

}

func (b *IpfsBackend) readJson(cid string, obj interface{}) error {
	reader, err := b.shell.Cat(cid)
	if err != nil {
		return err
	}
	defer reader.Close()
	dec := json.NewDecoder(reader)
	return dec.Decode(obj)
}

func (b *IpfsBackend) writeJson(obj interface{}) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := enc.Encode(obj)
	if err != nil {
		return "", err
	}
	return b.shell.Add(&buf)
}

func (b *IpfsBackend) SavePost(post Post, user User) error {
	postcid, err := b.writeJson(&post)
	if err != nil {
		return err
	}
	user.LastPost = postcid
	return b.SaveUser(user)
}

func (b *IpfsBackend) CatReader(cid string) (io.ReadCloser, error) {
	return b.shell.Cat(cid)
}

func (b *IpfsBackend) Cat(cid string) (string, error) {
	contentreader, err := b.shell.Cat(cid)
	if err != nil {
		return "", fmt.Errorf("can't get content %s: %w", cid, err)
	}
	defer contentreader.Close()
	bytes, err := ioutil.ReadAll(contentreader)
	if err != nil {
		return "", fmt.Errorf("can't get content %s: %w", cid, err)
	}
	return string(bytes), nil
}

func (b *IpfsBackend) Add(r io.Reader) (string, error) {
	return b.shell.Add(r)
}

func AddString(backend Backend, content string) (string, error) {
	return backend.Add(strings.NewReader(content))
}

const ipnsprefix = "/ipns/"

func (b *IpfsBackend) GetUserById(usercid string) (User, error) {

	//todo resolve ens address https://github.com/wealdtech/go-ens and infura

	//does this do anything?
	link, err := dnslink.Resolve(usercid)
	if err != nil && strings.HasPrefix(link, ipnsprefix) {
		usercid = link[len(ipnsprefix):]
	}
	var user User
	usercid, err = b.shell.Resolve(usercid)
	if err != nil {
		if strings.Contains(err.Error(), "could not resolve name") {
			return user, nil //bad idea. too late!
		}
		return user, fmt.Errorf("can't resolve key: %w", err)

	}
	err = b.readJson(usercid, &user)
	log.Printf("got user %s/%s", user.PublicName, usercid)
	return user, err
}

func (b *IpfsBackend) GetUserId() string {
	return b.key.Id
}

func (b *IpfsBackend) EnsureKey(ctx context.Context, keyName string) (*ipfs.Key, error) {
	keys, err := b.shell.KeyList(ctx)
	if err != nil {
		return nil, fmt.Errorf("Can't get keys %s", err)
	}
	var key *ipfs.Key
	for _, k := range keys {
		if k.Name == keyName {
			key = k
		}
	}
	if key == nil {
		key, err = b.shell.KeyGen(ctx, keyName)
		if err != nil {
			return nil, fmt.Errorf("Can't create keys %s", keyName)
		}
	}
	return key, nil
}

func (b *IpfsBackend) ExportKey() ([]byte, error) {
	privatekey, err := b.keystore.Get(b.key.Name)
	if err != nil {
		return nil, err
	}
	return crypto.MarshalPrivateKey(privatekey)
}

func (b *IpfsBackend) ImportKey(kpbytes []byte) error {

	privatekey, err := crypto.UnmarshalPrivateKey(kpbytes)
	if err != nil {
		return err
	}
	//overwrites exiting
	return b.keystore.Put(b.key.Name, privatekey)
}

func (b *IpfsBackend) SaveUser(user User) error {
	usercid, err := b.writeJson(&user)
	if err != nil {
		return err
	}

	//too slow to block responses
	go func() {
		resp, err := b.shell.PublishWithDetails(usercid, b.key.Name, 0, 0, false)
		if err != nil {
			log.Printf("Failed to post user %s to %s\n", usercid, b.key.Name)
			return
		}
		log.Printf("Posted user %s to %s\n", usercid, resp.Name)
	}()
	return nil
}

//offset
func (b *IpfsBackend) GetPosts(user User, count int) ([]Post, error) {
	head := user.LastPost
	var posts []Post
	for i := 0; head != "" && i < count; i++ {
		var post Post
		if err := b.readJson(head, &post); err != nil {
			return posts, fmt.Errorf("can't resolve content %s: %w", head, err)
		}
		posts = append(posts, post)
		head = post.Previous
	}
	log.Printf("got %d posts from %s", len(posts), user.PublicName)
	return posts, nil
}
