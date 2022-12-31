package zebu

import (
	"testing"
)

func TestResolveDNS(t *testing.T) {
	testdomain := "johnwilkes.northbriton.net"
	a, err := Resolve(testdomain)
	if err != nil {
		t.Fatal("didn't find dns")
	}
	if a != account {
		t.Fatalf("%s!=%s", a, account)
	}

}

func TestResolveEns(t *testing.T) {
	testdomain := "northbriton.eth"
	a, err := Resolve(testdomain)
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

func TestResolveNoop(t *testing.T) {
	a, err := Resolve(account)
	if err != nil {

		t.Fatalf("didn't noop, %v", err)
	}
	if a != account {
		t.Fatalf("%s!=%s", a, account)
	}
}
