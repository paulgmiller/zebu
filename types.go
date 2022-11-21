package main

import (
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/storyicon/sigverify"
)

//this is an ipns replacement so we can use ethereum signatures.
//https://github.com/ipfs/specs/blob/main/ipns/IPNS_PUBSUB.md
//https://github.com/ipfs/specs/blob/main/ipns/IPNS.md#record-serialization-format"
//go validation would be good here.
type UserNameRecord struct {
	CID       string
	Sequence  uint64
	Signature string `json:"Signature,omitempty"`
	PubKey    string //should we use bytes?
}

func (unr UserNameRecord) Validate() bool {
	clone := unr
	clone.Signature = ""

	data, err := json.Marshal(clone)
	if err != nil {
		log.Printf("couldn't marshal unr %s", err)
		return false
	}

	//{"CID":"Qmf7u5D4xAiAALdBTaFhsmU29PycWgZrZStV4Sv83n4icQ","Sequence":1,"PubKey":"0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"}
	//{"CID":"QmYpdmbS3m677XLjixE6YkeMxCcnAvxmksWiubK4pigiFw","Sequence":1,"PubKey":"0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"}
	//https://github.com/ethereum/go-ethereum/blob/b628d7276624c2d8ea7dd97d2259a2c2fce7d3cc/accounts/accounts.go#L197

	//https://ethereum.stackexchange.com/questions/45580/validating-go-ethereum-key-signature-with-ecrecover
	//https://github.com/storyicon/sigverify
	//https://github.com/ethereum/go-ethereum/blob/1c737e8b6da2b14111f8224ef3f385b1fe0cd8b9/crypto/signature_cgo.go#L32

	sig := unr.Signature
	//why doesn't hex.DecodeString do this for me?
	if strings.HasPrefix(sig, "0x") {
		sig = unr.Signature[2:]
	}

	sigbytes, err := hex.DecodeString(sig)
	if err != nil {
		log.Printf("sig wasn't hex %s", err)
		return false
	}
	addr, err := sigverify.EcRecover(data, sigbytes)
	if err != nil {
		log.Printf("got error recovering addr %s", err)
		return false
	}
	//this is still wrong recovered 0xF2Fafe8D71E17D9d197D496d29AcF4bbBd066eC4 known addr 0xCbd6073f486714E6641bf87c22A9CEc25aCf5804
	log.Printf("recovered %s known addr %s", addr.Hex(), unr.PubKey)
	return addr.Hex() == unr.PubKey

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
