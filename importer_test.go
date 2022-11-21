package main

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

type MokUserBackend struct{}

func (MokUserBackend) GetUserById(usercid string) (User, error) {
	panic("not ready ")
}
func (MokUserBackend) PublishUser(UserNameRecord) error { return nil }

func (MokUserBackend) SaveUserCid(user User) (UserNameRecord, error) {
	return UserNameRecord{
		CID:      "Qmd8fBSQeJ2MNkALQiLCFihymSAM4o7i13VnEJSAofAZWb",
		Sequence: 6,
		PubKey:   user.PublicName,
	}, nil
}

func TestImportsigning(t *testing.T) {

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	user := User{PublicName: crypto.PubkeyToAddress(key.PublicKey).Hex(), LastPost: "blah"}
	err = publishWithKey(user, MokUserBackend{}, key)
	if err != nil {
		t.Fatal(err)
	}

}
