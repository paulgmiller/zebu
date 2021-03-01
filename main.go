package main

import (
	"fmt"
	"os"
	"strings"

	ipfs "github.com/ipfs/go-ipfs-api"
)

func main() {
	fmt.Println("weee")
	ipfs := ipfs.NewShell("localhost:5001")
	cid, err := ipfs.Add(strings.NewReader("hunky doery"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	fmt.Printf("added %s", cid)
}
