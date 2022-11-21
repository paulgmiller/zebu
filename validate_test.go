package main

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/storyicon/sigverify"
)

const account = "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"

func TestSigVerify(t *testing.T) {
	const (
		testsig  = "0x3af8f094d62c1db889050993f803d59844f90cb2eec606353001a38c2a55c43934e5118b829afd8575a19492eb9262c2bbea1d73886ef9772c4f8e75e1d162351c"
		testdata = "TESTTEST"
	)

	sigbytes, err := hex.DecodeString(testsig[2:])
	if err != nil {
		t.Fatalf("sig wasn't hex %s", err)
	}
	addr, err := sigverify.EcRecover([]byte(testdata), sigbytes)
	if err != nil {
		t.Fatalf("got error recovrge addr %s", err)
	}
	if addr.Hex() != account {
		t.Fatalf("recovered %s known addr %s", addr.Hex(), account)
	}
}

func TestUNRValdiate(t *testing.T) {
	unr := UserNameRecord{
		CID:       "QmP95DscnxiNzzDJ7wcivJNKe1xNCRzxh8Td9Uo5focKpZ",
		Sequence:  1,
		Signature: "0x45480889de60205eb8acc159f812a6d965c6c7b303d0efaf91ab211132dc9bd70be2f8c5f85d913226f4259f1da021c0dd8b9c4a1f677a4e52cb3a8cc9e209361b",
		PubKey:    "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804",
	}

	if !unr.Validate() {
		t.Fatalf("didn't validate")
	}

}

func TestUNRValdiate2(t *testing.T) {
	//{"CID":"Qmd8fBSQeJ2MNkALQiLCFihymSAM4o7i13VnEJSAofAZWb",
	//"Sequence":6,
	//"Signature":"0x59657ed9783c2fcce688c93b2f3a2196ce7fd07b2e2cca52c3a1bcde97db68136353d83304a27d0fd4824bbedcc75834e191034ba1d6b409d4e2f3c5e742f0051b",
	//"PubKey":"0xCbd6073f486714E6641bf87c22A9CEc25aCf5804"
	unr := UserNameRecord{
		CID:       "Qmd8fBSQeJ2MNkALQiLCFihymSAM4o7i13VnEJSAofAZWb",
		Sequence:  6,
		Signature: "0x59657ed9783c2fcce688c93b2f3a2196ce7fd07b2e2cca52c3a1bcde97db68136353d83304a27d0fd4824bbedcc75834e191034ba1d6b409d4e2f3c5e742f0051b",
		PubKey:    "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804",
	}

	if !unr.Validate() {
		t.Fatalf("didn't validate")
	}

	unr.Signature = "59657ed9783c2fcce688c93b2f3a2196ce7fd07b2e2cca52c3a1bcde97db68136353d83304a27d0fd4824bbedcc75834e191034ba1d6b409d4e2f3c5e742f0051b"
	if !unr.Validate() {
		t.Fatalf("didn't validate 0x prefix")
	}

}

func TestSigning(t *testing.T) {

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	unr := UserNameRecord{
		CID:      "Qmd8fBSQeJ2MNkALQiLCFihymSAM4o7i13VnEJSAofAZWb",
		Sequence: 6,
		PubKey:   crypto.PubkeyToAddress(key.PublicKey).Hex(),
	}
	unr.Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	if !unr.Validate() {
		t.Fatalf("failed to validate %v", unr)
	}
}
