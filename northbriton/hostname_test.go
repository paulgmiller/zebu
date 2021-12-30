package main

import (
	"fmt"
	"testing"
)

func TestMarshal(t *testing.T) {
	fmt.Printf("%v", domainregex)
	if domainregex.MatchString("brad/foobar") {
		t.Fatalf("what the holy hell")
	}

}
