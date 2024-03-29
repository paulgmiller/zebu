package main

import (
	"context"
	"flag"
	"log"
	"paulgmiller/zebu/zebu"
	//https://pkg.go.dev/github.com/ipfs/go-ipfs-api#Key
)

const nobody = "nobody"

func main() {
	//https://github.com/urfave/cli/blob/master/docs/v2/manual.md#subcommands
	//https://github.com/spf13/viper
	resolve := flag.String("resolve", nobody, "look them up")
	opmlpath := flag.String("import", "", "import an opml feed")
	//unfollow := flag.String("unfollow", "nobody", "remove somone to your follows")
	flag.Parse()
	ctx := context.Background()

	if *resolve != nobody {
		hash, err := zebu.Resolve(*resolve)
		if err != nil {
			panic(err)
		}
		log.Println(hash)
		return
	}

	backend := zebu.NewIpfsBackend(ctx)

	if *opmlpath != "" {
		log.Printf("opmlpath %s", *opmlpath)

		imports, err := Import(ctx, *opmlpath)
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Printf("imported %v", imports)
		return
	}

	serve(backend)
}
