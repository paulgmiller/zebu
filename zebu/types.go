package zebu

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
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

	//this magic is in sigverify and signer/core/signed_data.go in go-ethereeum.
	if sigbytes[64] != 27 && sigbytes[64] != 28 {
		log.Printf("invalid Ethereum signature (V is not 27 or 28)")
		return false
	}
	sigbytes[64] -= 27 // Transform yellow paper V from 27/28 to 0/1

	pubkey, err := crypto.SigToPub(accounts.TextHash(data), sigbytes)

	//pubkey, err := crypto.Ecrecover(accounts.TextHash(data), sigbytes)
	if err != nil {
		log.Printf("got error recovering addr %s", err)
		return false
	}
	addr := crypto.PubkeyToAddress(*pubkey)
	//log.Printf("recovered %s known addr %s", addr.Hex(), unr.PubKey)
	return addr.Hex() == unr.PubKey

}

func (unr *UserNameRecord) Sign(privatekey *ecdsa.PrivateKey) error {
	junr, err := json.Marshal(unr)
	if err != nil {
		return fmt.Errorf("could not marshal %v, %w", unr, err)
	}

	sig, err := crypto.Sign(accounts.TextHash(junr), privatekey)
	if err != nil {
		return fmt.Errorf("could not sign  %s, %w", junr, err)
	}

	//magic see github.com/ethereum/go-ethereum@v1.10.20/signer/core/signed_data.go
	sig[64] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	unr.Signature = hex.EncodeToString(sig)
	return nil
}

//todo better names for display and public names that deosn't break back compat
type User struct {
	LastPost     string
	Follows      []string //store ens/dns display names and resolve when needed
	Likes        string   //points to a LikeChunk
	DisplayName  string   //ens or dns name
	PublicName   string   //public key
	ImportSource string   `json:"ImportSource,omitempty"`
}

type LikeChunk struct {
	Previous string
	Likes    []string
}

func (u *User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.PublicName
}

func (u *User) PublicKey() string {
	return u.PublicName
}

//ugh why doesn't this exist.
func (u *User) Follow(user string) {
	for _, f := range u.Follows {
		if f == user {
			return
		}
	}
	u.Follows = append(u.Follows, user)
}

//previous, contentm and images are all CIDS but we don't recurse automatically using ipfs because we don't want pin all history.
type Post struct {
	Previous string
	Content  string
	Images   []string  //this makes it hard to do images inline? don't care?
	Created  time.Time //can't actually trust this
	Author   string    //publicname?
}

//this is never meant to be a backend  contract and just a ui helper.
type FetchedPost struct {
	Post
	RenderedContent template.HTML
	Author          string //this can be a lie if I repost someone elses thing.
}

func (fp FetchedPost) PrettyCreated() string {

	prettyTime := fp.Post.Created.Format("Jan 2, 2006")
	ago := time.Now().Sub(fp.Post.Created)
	if ago < 24*time.Hour {
		//wrong need to shave hours off minutes
		rounded := ago.Round(time.Minute)
		pretty := strings.Replace(rounded.String(), "0s", "", 1)
		pretty = strings.Replace(pretty, "h", "h ", 1)
		prettyTime = fmt.Sprintf("%s ago", pretty)
	}
	return prettyTime
}
