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
	SaveUser(user User) chan error
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

	//keys
	key      *ipfs.Key
	keystore keystore.Keystore

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
		shell:    shell,
		keystore: ks,
	}

	if backend.key, err = backend.EnsureKey(ctx, keyName); err != nil {
		log.Fatal(err.Error())
	}

	if found, _ := ks.Has(keyName); !found {
		log.Fatal("Coudn't find key in keystore")
	}
	//TODO need a way to communicate failures back
	if err := backend.listen(ctx); err != nil {
		log.Fatal("coudlnt set up listener")
	}
	return backend
}

func (b *IpfsBackend) listen(ctx context.Context) error {
	sub, err := b.shell.PubSubSubscribe("/zebu")
	if err != nil {
		return err
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

func (b *IpfsBackend) GetUserById(usercid string) (User, error) {

	//todo resolve ens address https://github.com/wealdtech/go-ens and infura

	//does this do anything?
	link, err := dnslink.Resolve(usercid)
	if err != nil && strings.HasPrefix(link, ipnsprefix) {
		usercid = link[len(ipnsprefix):]
	}

	//temporary remove when we updte dns?
	if key, _ := b.getKey(context.TODO(), usercid); key != nil {
		log.Printf("resolved %s -> %s", usercid, key.Id)
		usercid = key.Id
	}

	var user User
	usercid, err = b.shell.Resolve(usercid)
	if err != nil {
		log.Printf("failed to resolve %s", usercid)
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

func (b *IpfsBackend) getKey(ctx context.Context, keyName string) (*ipfs.Key, error) {
	keys, err := b.shell.KeyList(ctx)
	if err != nil {
		return nil, fmt.Errorf("Can't get keys %s", err)
	}
	for _, k := range keys {
		if k.Name == keyName {
			return k, nil
		}
	}
	return nil, nil
}

func (b *IpfsBackend) EnsureKey(ctx context.Context, keyName string) (*ipfs.Key, error) {
	key, err := b.getKey(ctx, keyName)
	if err != nil {
		return nil, err
	}

	if key == nil {
		log.Printf("generating %s", keyName)
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

func (b *IpfsBackend) SaveUser(user User) chan error {
	result := make(chan error, 1)
	unr, err := b.SaveUserCid(user)
	if err != nil {
		result <- err
		return result
	}
	//too slow to block responses in most cases....
	go func() {

		resp, err := b.shell.PublishWithDetails(unr.CID, b.key.Name, 0, 0, false)
		if err != nil {
			log.Printf("Failed to post user %s to %s\n", unr.CID, b.key.Name)
			result <- err
			return
		}
		log.Printf("Posted user %s to %s:%s\n", unr.CID, b.key.Name, resp.Name)
		result <- nil
	}()
	return result
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
