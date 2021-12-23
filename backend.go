package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"

	"github.com/ipfs/go-dnslink"
	ipfs "github.com/ipfs/go-ipfs-api"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/mitchellh/go-homedir"
	"github.com/moby/moby/pkg/namesgenerator"
)

type IpfsBackend struct {
	shell     *ipfs.Shell
	key       *ipfs.Key
	namecache map[string]string
	cahcelock sync.Mutex
	keystore  keystore.Keystore
}

func NewIpfsBackend(ctx context.Context, keyName string) *IpfsBackend {

	shell := ipfs.NewShell("localhost:5001")
	if !shell.IsUp() {
		log.Fatal("Ipfs not fond on localhost:5001 please install https://docs.ipfs.io/install/command-line/#official-distributions")
	}
	keys, err := shell.KeyList(ctx)
	if err != nil {
		log.Fatalf("Can't get keys %s", err)
	}
	var key *ipfs.Key
	for _, k := range keys {
		if k.Name == keyName {
			key = k
		}
	}
	if key == nil {
		key, err = shell.KeyGen(ctx, keyName)
		if err != nil {
			log.Fatalf("Can't create keys %s", keyName)
		}
	}
	keystoredir, _ := homedir.Expand("~/.ipfs/keystore")
	ks, err := keystore.NewFSKeystore(keystoredir)
	if err != nil {
		log.Fatalf("Can't create keystore %s", keyName)
	}
	if found, _ := ks.Has(key.Name); !found {
		log.Fatal("Coudn't find key in keystore")
	}

	return &IpfsBackend{
		shell:     shell,
		key:       key,
		namecache: map[string]string{},
		keystore:  ks,
	}

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

type Backend interface {
	GetUserById(usercid string) (User, error)
	GetUserId() string
	SaveUser(user User) error
	GetPosts(user User, count int) ([]Post, error)
	SavePost(post Post, user User) error
	//too low level?
	Cat(cid string) (string, error)
	Add(content string) (string, error)

	ExportKey() ([]byte, error)
}

func (b *IpfsBackend) SavePost(post Post, user User) error {
	postcid, err := b.writeJson(&post)
	if err != nil {
		return err
	}
	user.LastPost = postcid
	return b.SaveUser(user)
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

func (b *IpfsBackend) Add(content string) (string, error) {
	cid, err := b.shell.Add(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	return cid, nil
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
			user.DisplayName = namesgenerator.GetRandomName(0)
			return user, nil //bad idea?
		}
		return user, fmt.Errorf("can't resolve key: %w", err)

	}
	err = b.readJson(usercid, &user)
	if user.DisplayName == "" {
		b.cahcelock.Lock()
		defer b.cahcelock.Unlock()
		displayname, ok := b.namecache[usercid]
		if !ok {
			displayname = namesgenerator.GetRandomName(0)
			b.namecache[usercid] = displayname
		}
		user.DisplayName = displayname
	}
	log.Printf("got user %s/%s", user.DisplayName, usercid)
	return user, err
}

func (b *IpfsBackend) GetUserId() string {
	return b.key.Id
}

//seperater interface?
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
	log.Printf("got %d posts from %s", len(posts), user.DisplayName)
	return posts, nil
}
