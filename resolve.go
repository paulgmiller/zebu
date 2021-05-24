package main

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	ens "github.com/wealdtech/go-ens/v3"
)

//other options https://eth.link/ dnslink

func ResolveEns(ensdomain string) {
	//obviusly need a light client but
	ethendpoint := os.Getenv("ETHENDPOINT")
	client, err := ethclient.Dial(ethendpoint)
	if err != nil {
		panic(err)
	}

	// Resolve a name to an address
	resolver, err := ens.NewResolver(client, ensdomain)

	if err != nil {
		panic(err)
	}
	hash, err := resolver.Contenthash()
	if err != nil {
		panic(err)
	}
	readablehash, err := ens.ContenthashToString(hash)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Content hash of %s is %s\n", ensdomain, readablehash)

	//usercid, err := ipfsShell.Resolve(*resolve)
	/*
		link, err := dnslink.Resolve(*ensdomain)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println(link)
		}
	*/
}
