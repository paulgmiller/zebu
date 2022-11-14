package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync"

	"github.com/ipfs/go-dnslink"
	ipfs "github.com/ipfs/go-ipfs-api"
)

type Backend interface {
	ContentBackend
}

type ContentBackend interface {
	GetUserById(usercid string) (User, error)
	//GetUserId() string
	SaveUserCid(user User) (UserNameRecord, error)
	GetPosts(user User, count int) ([]Post, error)
	SavePost(post Post) (string, error)
	//too low level?
	Cat(cid string) (string, error) //remove with helper method.
	CatReader(cid string) (io.ReadCloser, error)
	Add(r io.Reader) (string, error)
}

type IpfsBackend struct {
	//content
	shell *ipfs.Shell

	//pubsub
	lock    sync.RWMutex
	records map[string]UserNameRecord
}

func NewIpfsBackend(ctx context.Context, keyName string) *IpfsBackend {

	//https: //github.com/ipfs/kubo/tree/master/docs/examples/kubo-as-a-library
	shell := ipfs.NewShell("localhost:5001")
	if !shell.IsUp() {
		log.Fatal("Ipfs not fond on localhost:5001 please install https://docs.ipfs.io/install/command-line/#official-distributions")
	}

	backend := &IpfsBackend{
		shell: shell,
	}

	//TODO need a way to communicate failures back
	if err := backend.listen(ctx); err != nil {
		log.Fatalf("coudlnt set up listener, %s", err)
	}
	return backend
}

const centraltopic = "zebu"

func (b *IpfsBackend) listen(ctx context.Context) error {
	sub, err := b.shell.PubSubSubscribe(centraltopic)
	if err != nil {
		return fmt.Errorf("failed to subsciribe to %s %w", centraltopic, err)
	}
	go func() {
		for {
			if ctx.Err() != nil {
				break
			}

			msg, err := sub.Next()
			if err != nil {
				log.Fatalf("subscription broke") //TODO what kind of errors should expect here Can we recver or should we tear down?
			}

			//would msg.sequence number replace
			unr := &UserNameRecord{}
			if err = json.Unmarshal(msg.Data, unr); err != nil {
				log.Printf("unserializable message %v", msg.Data)
				continue
			}

			if !unr.Validate() {
				//be nice to track peers and stop taking invalid messages from bad ones. (sigh reimplementing ipns I would guess)
				log.Printf("invalid message %v", unr)
				continue
			}

			b.lock.RLock()
			defer b.lock.RUnlock()
			existing := b.records[unr.PubKey]
			if unr.Sequence > existing.Sequence {
				b.lock.Lock()
				b.records[unr.PubKey] = *unr
				b.lock.Unlock()
			}
		}
	}()
	return nil
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

func (b *IpfsBackend) SavePost(post Post) (string, error) {
	return b.writeJson(&post)
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

//so a user id could be ens/dns/or ethereum public key
func (b *IpfsBackend) GetUserById(userid string) (User, error) {

	//todo resolve ens address https://github.com/wealdtech/go-ens and infura
	//but to start use ResolveEthLink/https://eth.link/

	link, err := dnslink.Resolve(userid)
	if err != nil && strings.HasPrefix(link, ipnsprefix) {
		userid = link[len(ipnsprefix):]
	}

	var user User
	b.lock.RLock()
	defer b.lock.RUnlock()
	userrecord, found := b.records[userid]
	if !found {
		return user, nil //bad idea. too late!
	}
	err = b.readJson(userrecord.CID, &user)
	log.Printf("got user %s/%s", user.PublicName, userrecord.CID)
	return user, err
}

func (b *IpfsBackend) SaveUserCid(user User) (UserNameRecord, error) {
	cid, err := b.writeJson(&user)
	if err != nil {
		return UserNameRecord{}, err
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	existing := b.records[user.PublicName]
	existing.Sequence += 1
	existing.PubKey = user.PublicName //just in case there was no existing
	existing.CID = cid
	return existing, nil
}

func (b *IpfsBackend) PublishUser(u UserNameRecord) error {
	ujsonbytes, err := json.Marshal(u)
	if err != nil {
		return err
	}
	ujson := string(ujsonbytes)

	//so to start with we'll publish everythig to one path to make everthing findable. Eventually that will explode
	if err := b.shell.PubSubPublish("/zebu", ujson); err != nil {
		return err
	}
	if b.shell.PubSubPublish("/zebu/"+string(u.PubKey), ujson); err != nil {
		return err
	}
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
	//log.Printf("got %d posts from %s", len(posts), user.PublicName)
	return posts, nil
}
