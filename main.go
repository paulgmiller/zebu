package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
)

func main() {
	//https://github.com/urfave/cli/blob/master/docs/v2/manual.md#subcommands
	keyName := flag.String("key", "zebu", "what ipns key are we using")
	resolve := flag.String("resolve", nobody, "look them up")
	opmlpath := flag.String("import", "", "import an opml feed")
	//unfollow := flag.String("unfollow", "nobody", "remove somone to your follows")
	flag.Parse()
	ctx := context.Background()

	if *resolve != nobody {
		hash, err := ResolveEthLink(*resolve)
		if err != nil {
			panic(err)
		}
		fmt.Println(hash)
		return
	}

	log.Printf("opmlpath %s", *opmlpath)

	if *opmlpath != "" {
		err := Import(ctx, *opmlpath)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	backend := NewIpfsBackend(ctx, *keyName)

	serve(backend)
}
