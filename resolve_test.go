package main

import (
	"testing"
)

func TestResolveDNS(t *testing.T) {
	testdomain := "johnwilkes.northbriton.net"
	a, err := ResolveDns(testdomain)
	if err != nil {
		t.Fatal("didn't find dns")
	}
	if a != account {
		t.Fatalf("%s!=%s", a, account)
	}

}

func TestResolveEns(t *testing.T) {
	testdomain := "northbriton.eth"
	a, err := ResolveEns(testdomain)
	if err != nil {
		if err == NoEthEndpoint {
			return
		}
		t.Fatalf("didn't find ens, %v", err)
	}
	if a != account {
		t.Fatalf("%s!=%s", a, account)
	}
}
