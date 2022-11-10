package main

import (
	"context"
	"flag"
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
		log.Println(hash)
		return
	}

	backend := NewIpfsBackend(ctx, *keyName)

	if *opmlpath != "" {
		log.Printf("opmlpath %s", *opmlpath)
		/*
			imports, err := Import(ctx, *opmlpath)
			if err != nil {
				log.Fatal(err.Error())
			}
			//user, err := backend.GetUserById(backend.GetUserId())
			if err != nil {
				log.Fatal(err.Error())
			}
			for _, i := range imports {
				user.Follow(i)
			}
			backend.SaveUser(user)
		*/
	}

	serve(backend)
}
