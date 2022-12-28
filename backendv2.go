package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	cidlib "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/multiformats/go-multiaddr"
)

type Backend interface {
	ContentBackend
	UserBackend
	Healthz
	RandomUsers(int) []string
}

type UserBackend interface {
	GetUserById(ctx context.Context, usercid string) (User, error)
	PublishUser(ctx context.Context, unr UserNameRecord) error
	SaveUserCid(ctx context.Context, user User) (UserNameRecord, error)
}

type Healthz interface {
	Healthz(context.Context) bool
}

type ContentBackend interface {
	GetPosts(ctx context.Context, user User, count int) ([]Post, error)
	SavePost(ctx context.Context, post Post) (string, error)
	//too low level? used for images currently
	Cat(ctx context.Context, cid string) (io.ReadCloser, error)
	Add(ctx context.Context, r io.Reader) (string, error)
}

var _ Backend = &IpfsBackend{}

type IpfsBackend struct {
	//content
	//shell *ipfs.Shell
	api *httpapi.HttpApi

	//pubsub caching layer.
	lock         sync.RWMutex
	records      map[string]UserNameRecord
	healthrecord path.Path
}

func NewIpfsBackend(ctx context.Context) *IpfsBackend {

	ipfsserver, found := os.LookupEnv("IPFS_SERVER")
	if !found {
		ipfsserver = "/ip4/127.0.0.1/tcp/5001"
	}

	//https: //github.com/ipfs/kubo/tree/master/docs/examples/kubo-as-a-library

	ipsaddr, err := multiaddr.NewMultiaddr(ipfsserver)
	if err != nil {
		log.Fatalf("failed to parse %s", ipfsserver)
	}
	ipfsapi, err := httpapi.NewApi(ipsaddr)
	if err != nil {
		log.Fatalf("failed to start api  %s", err)
	}

	hr, err := ipfsapi.Unixfs().Add(ctx, files.NewBytesFile([]byte("healthz")))
	if err != nil {
		log.Fatalf("failed to store healthz %s", err)
	}

	backend := &IpfsBackend{
		api:          ipfsapi,
		records:      map[string]UserNameRecord{},
		healthrecord: hr,
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
	for k := range b.records {
		if n <= 0 {
			break
		}
		users = append(users, k)
		n -= 1
	}
	return users
}

func (b *IpfsBackend) Healthz(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := b.api.Unixfs().Get(ctx, b.healthrecord)
	return err == nil
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
	sub, err := b.api.PubSub().Subscribe(ctx, centraltopic)
	//sub, err := b.shell.PubSubSubscribe(centraltopic)
	if err != nil {
		return fmt.Errorf("failed to subsciribe to %s %w", centraltopic, err)
	}
	go func() {
		for {
			if ctx.Err() != nil {
				break
			}

			msg, err := sub.Next(ctx)
			if err != nil {
				log.Fatalf("subscription broke") //TODO what kind of errors should expect here Can we recver or should we tear down?
			}

			//would msg.sequence number replace
			unr := &UserNameRecord{}
			if err = json.Unmarshal(msg.Data(), unr); err != nil {
				log.Printf("unserializable message %v", msg.Data())
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
					//if err := b.shell.FilesWrite(context.TODO(), usertopic, bytes.NewReader(msg.Data), ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Parents(true)); err != nil {
					//b.api doesn't have mutable files yet.
					err = os.WriteFile("users/"+string(unr.PubKey), msg.Data(), 0600)
					if err != nil {
						log.Printf("failed to save %s", unr.PubKey)
					}
					log.Printf("wrote to %s", usertopic)
				}
			}()
		}
	}()
	return nil
}

func CatString(ctx context.Context, b ContentBackend, cidr string) (string, error) {

	r, err := b.Cat(ctx, cidr)
	if err != nil {
		return "", err
	}
	builder := &strings.Builder{}
	if _, err = io.Copy(builder, r); err != nil {
		return "", err
	}
	return builder.String(), nil

}

func (b *IpfsBackend) republishRecords(ctx context.Context) {

	go func() {
		for {
			if ctx.Err() != nil {
				break
			}

			files, err := ioutil.ReadDir("users")
			if err != nil {
				log.Fatalf("could't list user storage: %s", err)
			}
			//users, err := b.shell.FilesLs(ctx, centraltopic, ipfs.FilesLs.Stat(true))
			if err != nil {
				log.Fatalf("could't list user storage: %s", err)
			}
			for _, f := range files {

				bytes, err := ioutil.ReadFile("users/" + f.Name())
				if err != nil {
					log.Fatalf("could't read user %s: %s", f.Name(), err)
				}

				usertopic := centraltopic + "/" + f.Name()

				//so to start with we'll publish everythig to one path to make everthing findable. Eventually that will explode
				if err := b.api.PubSub().Publish(ctx, centraltopic, bytes); err != nil {
					log.Printf("failed to publish to %s, %s", usertopic, err)
				}
				if err := b.api.PubSub().Publish(ctx, usertopic, bytes); err != nil {
					log.Printf("failed to publish to %s, %s", usertopic, err)
				}
			}
			time.Sleep(10 * time.Second) //is this impolite
		}
	}()
}

func (b *IpfsBackend) loadRecords(ctx context.Context) {

	if _, err := os.Stat("users"); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir("users", os.ModePerm); err != nil {
			log.Fatalf("couldn't make dir: %s", err)
		}
	}

	//users, err := b.shell.FilesLs(ctx, centraltopic, ipfs.FilesLs.Stat(true))
	files, err := os.ReadDir("users")
	if err != nil {
		log.Fatalf("could't list user storage: %s", err)
	}
	//users, err := b.api.Unixfs().Ls(ctx, path.New(centraltopic))
	log.Printf("got %d users", len(files))
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, f := range files {
		var unr UserNameRecord
		f.Info()
		bytes, err := ioutil.ReadFile("users/" + f.Name())
		if err != nil {
			log.Fatalf("could't read user %s: %s", f.Name(), err)
		}
		err = json.Unmarshal(bytes, &unr)
		if err != nil {
			log.Fatalf("could't read user %s: %s", f.Name(), err)
		}
		b.records[unr.PubKey] = unr
	}
}

func (b *IpfsBackend) readJson(cidstr string, obj interface{}) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	cid, err := cidlib.Parse(cidstr)
	if err != nil {
		return fmt.Errorf("faild to parse cidr %w", err)
	}

	entry, err := b.api.Unixfs().Get(ctx, path.IpfsPath(cid))
	if err != nil {
		return fmt.Errorf("faild to get object %s, %w", path.IpfsPath(cid), err)
	}
	f := files.ToFile(entry)
	if f == nil {
		return fmt.Errorf("%s not a file", cidstr)
	}

	defer f.Close()
	dec := json.NewDecoder(f)
	return dec.Decode(obj)
}

func (b *IpfsBackend) writeJson(obj interface{}) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := enc.Encode(obj)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	path, err := b.api.Unixfs().Add(ctx, files.NewBytesFile(buf.Bytes()))
	if err != nil {
		return "", err
	}
	return path.Cid().String(), nil
}

func (b *IpfsBackend) SavePost(ctx context.Context, post Post) (string, error) {
	return b.writeJson(&post)
}

func (b *IpfsBackend) Cat(ctx context.Context, cidstr string) (io.ReadCloser, error) {
	cid, err := cidlib.Parse(cidstr)
	if err != nil {
		return nil, err
	}
	entry, err := b.api.Unixfs().Get(context.TODO(), path.IpfsPath(cid))
	if err != nil {
		return nil, fmt.Errorf("faild to get object %s, %w", path.IpfsPath(cid), err)
	}
	f := files.ToFile(entry)
	if f == nil {
		return nil, fmt.Errorf("%s not a file", cidstr)
	}

	return f, err
}

func (b *IpfsBackend) Add(ctx context.Context, r io.Reader) (string, error) {
	path, err := b.api.Unixfs().Add(ctx, files.NewReaderFile(r))
	if err != nil {
		return "", err
	}
	return path.Cid().String(), nil
}

func AddString(ctx context.Context, backend Backend, content string) (string, error) {
	return backend.Add(ctx, strings.NewReader(content))
}

//so a user id could be ens/dns/or ethereum public key
func (b *IpfsBackend) GetUserById(ctx context.Context, userid string) (User, error) {

	//todo resolve ens address https://github.com/wealdtech/go-ens and infura
	//but to start use ResolveEthLink/ https://eth.link/

	b.lock.RLock()
	userrecord, found := b.records[userid]
	b.lock.RUnlock()
	if !found {
		return User{PublicName: userid}, nil //bad idea. too late!
	}
	var user User
	fmt.Printf("reading json for %s ", userrecord.CID)
	err := b.readJson(userrecord.CID, &user)
	fmt.Printf("go error %s", err)
	return user, err
}

func (b *IpfsBackend) SaveUserCid(ctx context.Context, user User) (UserNameRecord, error) {
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

func (b *IpfsBackend) PublishUser(ctx context.Context, u UserNameRecord) error {
	if !u.Validate() {
		return fmt.Errorf("Invalid user %v", u)
	}

	ujsonbytes, err := json.Marshal(u)
	if err != nil {
		return err
	}
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

	//if _, err := b.api.Unixfs().Add(ctx, files.NewBytesFile(ujsonbytes)); err != nil {
	//if err := b.shell.FilesWrite(context.TODO(), usertopic, bytes.NewReader(ujsonbytes), ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Parents(true)); err != nil {
	if err = os.WriteFile("users/"+string(u.PubKey), ujsonbytes, 0600); err != nil {
		log.Printf("failed to write to %s, %s", usertopic, err)
		return err
	}
	log.Printf("wrote to %s", usertopic)
	//so to start with we'll publish everythig to one path to make everthing findable. Eventually that will explode

	if err := b.api.PubSub().Publish(ctx, centraltopic, ujsonbytes); err != nil {
		log.Printf("failed to publish to %s, %s", usertopic, err)
	}
	if err := b.api.PubSub().Publish(ctx, usertopic, ujsonbytes); err != nil {
		log.Printf("failed to publish to %s, %s", usertopic, err)
	}

	return nil
}

//offset
func (b *IpfsBackend) GetPosts(ctx context.Context, user User, count int) ([]Post, error) {
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
