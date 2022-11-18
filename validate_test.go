package main

import (
	"encoding/hex"
	"testing"

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
	unr := UserNameRecord{
		CID:       "Qmct7qVHMMpdsxmSsrY3ginXY3PAVYaZcpeA3UX8WXYw1q",
		Sequence:  6,
		Signature: "0x7da83c9cfd046464a95c1d970410069edd8ba483dc19e40c39497a7b6ee4012d29ce77165580ad7fd0aab7f714699328d9b4b1846d724e1319ee407e23ea832e1c",
		PubKey:    "0xCbd6073f486714E6641bf87c22A9CEc25aCf5804",
	}

	if !unr.Validate() {
		t.Fatalf("didn't validate")
	}

}
