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
	"sync"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

type Backend interface {
	ContentBackend
	UserBackend
	Healthz
	RandomUsers(int) []string
}

type UserBackend interface {
	GetUserById(usercid string) (User, error)
	PublishUser(UserNameRecord) error
	//GetUserId() string
	SaveUserCid(user User) (UserNameRecord, error)
}

type Healthz interface {
	Healthz() bool
}

type ContentBackend interface {
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

	//pubsub caching layer.
	lock    sync.RWMutex
	records map[string]UserNameRecord
}

func NewIpfsBackend(ctx context.Context) *IpfsBackend {

	ipfsserver, found := os.LookupEnv("IPFS_SERVER")
	if !found {
		ipfsserver = "localhost:5001"
	}

	//https: //github.com/ipfs/kubo/tree/master/docs/examples/kubo-as-a-library
	shell := ipfs.NewShell(ipfsserver)
	if !shell.IsUp() {
		log.Fatal("Ipfs not fond on localhost:5001 please install https://docs.ipfs.io/install/command-line/#official-distributions")
	}

	backend := &IpfsBackend{
		shell:   shell,
		records: map[string]UserNameRecord{},
	}

	log.Print("loading records")
	backend.loadRecords(ctx)
	backend.republishRecords(ctx)

	//TODO need a way to communicate failures back
	if err := backend.listen(ctx); err != nil {
		log.Fatalf("coudlnt set up listener, %s", err)
	}
	return backend
}

func (b *IpfsBackend) RandomUsers(n int) []string {
	b.lock.RLock()
	defer b.lock.RUnlock()
	users := []string{}
	for k, _ := range b.records {
		if n <= 0 {
			break
		}
		users = append(users, k)
		n -= 1
	}
	return users
}

func (b *IpfsBackend) Healthz() bool {
	return b.shell.IsUp()
}

const centraltopic = "/zebu"

/* This is basically reimplmenting ipns on top of pubsub
https://github.com/ipfs/specs/blob/main/ipns/IPNS_PUBSUB.md#layering-persistence-onto-libp2p-pubsub
we could dig down into the dht directly.
DEPRECATED SUBCOMMANDS
  ipfs dht findpeer <peerID>...   - Find the multiaddresses associated with a Peer ID.
  ipfs dht findprovs <key>...     - Find peers that can provide a specific value, given a key.
  ipfs dht get <key>...           - Given a key, query the routing system for its best value.
  ipfs dht provide <key>...       - Announce to the network that you are providing given values.
  ipfs dht put <key> <value-file> - Write a key/value pair to the routing system.
*/

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

			func() {
				b.lock.Lock()
				defer b.lock.Unlock()
				existing := b.records[unr.PubKey]
				if unr.Sequence > existing.Sequence {
					log.Printf("update is new %s", unr.PubKey)
					b.records[unr.PubKey] = *unr
					usertopic := centraltopic + "/" + string(unr.PubKey)
					if err := b.shell.FilesWrite(context.TODO(), usertopic, bytes.NewReader(msg.Data), ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Parents(true)); err != nil {
						log.Printf("failed to save %s", unr.PubKey)
					}
					log.Printf("wrote to %s", usertopic)
				}
			}()
		}
	}()
	return nil
}

func (b *IpfsBackend) republishRecords(ctx context.Context) {

	go func() {
		for {
			if ctx.Err() != nil {
				break
			}

			users, err := b.shell.FilesLs(ctx, centraltopic, ipfs.FilesLs.Stat(true))
			if err != nil {
				log.Fatalf("could't list user storage: %s", err)
			}
			for _, u := range users {

				json, err := b.Cat(u.Hash)
				if err != nil {
					log.Printf("couldn't read %s: %s", u.Name, u.Hash)
				}

				usertopic := centraltopic + "/" + u.Name

				//so to start with we'll publish everythig to one path to make everthing findable. Eventually that will explode
				if err := b.shell.PubSubPublish(centraltopic, json); err != nil {
					log.Printf("failed to publish to %s, %s", usertopic, err)
				}
				if err := b.shell.PubSubPublish(usertopic, json); err != nil {
					log.Printf("failed to publish to %s, %s", usertopic, err)
				}
			}
			time.Sleep(10 * time.Second) //is this impolite
		}
	}()
}

func (b *IpfsBackend) loadRecords(ctx context.Context) {

	if err := b.shell.FilesMkdir(ctx, centraltopic, ipfs.FilesMkdir.Parents(true)); err != nil {
		log.Fatalf("count't init user storage: %s", err)
	}

	users, err := b.shell.FilesLs(ctx, centraltopic, ipfs.FilesLs.Stat(true))
	if err != nil {
		log.Fatalf("could't list user storage: %s", err)
	}
	log.Printf("got %d users", len(users))
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, u := range users {
		var unr UserNameRecord
		if err := b.readJson(u.Hash, &unr); err != nil {
			log.Printf("couldn't read %s: %s", u.Name, u.Hash)
		}
		b.records[unr.PubKey] = unr
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

//so a user id could be ens/dns/or ethereum public key
func (b *IpfsBackend) GetUserById(userid string) (User, error) {

	//todo resolve ens address https://github.com/wealdtech/go-ens and infura
	//but to start use ResolveEthLink/ https://eth.link/

	b.lock.RLock()
	userrecord, found := b.records[userid]
	b.lock.RUnlock()
	if !found {
		return User{PublicName: userid}, nil //bad idea. too late!
	}
	var user User
	err := b.readJson(userrecord.CID, &user)
	return user, err
}

func (b *IpfsBackend) SaveUserCid(user User) (UserNameRecord, error) {
	cid, err := b.writeJson(&user)
	if err != nil {
		return UserNameRecord{}, err
	}
	b.lock.RLock()
	existing := b.records[user.PublicName]
	b.lock.RUnlock()
	existing.Sequence += 1
	existing.PubKey = user.PublicName //just in case there was no existing
	existing.CID = cid
	existing.Signature = "" //no longer valid
	return existing, nil
}

func (b *IpfsBackend) PublishUser(u UserNameRecord) error {
	if !u.Validate() {
		return fmt.Errorf("Invalid user %v", u)
	}

	ujsonbytes, err := json.Marshal(u)
	if err != nil {
		return err
	}
	ujson := string(ujsonbytes)
	{
		b.lock.Lock()
		defer b.lock.Unlock()
		old, found := b.records[u.PubKey]
		if found && old.Sequence > u.Sequence {
			return fmt.Errorf("found newer record with sequence %d", old.Sequence)
		}
		//some sort of dead lock
		b.records[u.PubKey] = u
	}
	usertopic := centraltopic + "/" + u.PubKey

	if err := b.shell.FilesWrite(context.TODO(), usertopic, bytes.NewReader(ujsonbytes), ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Parents(true)); err != nil {
		log.Printf("failed to write to %s, %s", usertopic, err)
		return err
	}
	log.Printf("wrote to %s", usertopic)
	//so to start with we'll publish everythig to one path to make everthing findable. Eventually that will explode
	if err := b.shell.PubSubPublish(centraltopic, ujson); err != nil {
		return err
	}
	if err := b.shell.PubSubPublish(usertopic, ujson); err != nil {
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
