package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"time"

	"golang.org/x/crypto/sha3"
)

//this is an ipns replacement so we can use ethereum signatures.
//https://github.com/ipfs/specs/blob/main/ipns/IPNS_PUBSUB.md
//https://github.com/ipfs/specs/blob/main/ipns/IPNS.md#record-serialization-format"
//go validation would be good here.
type UserNameRecord struct {
	CID       string
	Sequence  uint64
	Signature []byte `json:"Signature,omitempty"`
	PubKey    string //should we use bytes?
}

func (unr UserNameRecord) Validate() bool {
	clone := unr
	clone.Signature = []byte{}

	data, err := json.Marshal(clone)
	if err != nil {
		log.Printf("couldn't marshal unr %s", err)
		return false
	}

	//TODO validate signature.
	//https://github.com/ethereum/go-ethereum/blob/b628d7276624c2d8ea7dd97d2259a2c2fce7d3cc/accounts/accounts.go#L197
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), string(data))
	keccak256 := sha3.NewLegacyKeccak256()
	if _, err := keccak256.Write([]byte(msg)); err != nil {
		log.Printf("couldn't hash unr %s", err)
		return false
	}
	return bytes.Equal(keccak256.Sum(nil), unr.Signature)

}

type User struct {
	LastPost    string
	Follows     []string
	DisplayName string //end or dns name?
	PublicName  string //public key
}

//ugh why doesn't this exist.
func (u *User) Follow(userCidr string) {
	for _, f := range u.Follows {
		if f == userCidr {
			return
		}
	}
	u.Follows = append(u.Follows, userCidr)
}

//previous, contentm and images are all CIDS but we don't recurse automatically using ipfs because we don't want pin all history.
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

const nobody = "nobody"
