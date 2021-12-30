package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
	ens "github.com/wealdtech/go-ens/v3"
)

//other options https://eth.link/

/*$ curl -H 'content-type: application/dns-json' 'https://eth.link/dns-query?type=TXT&name=wealdtech.eth'
{
	"AD":true,"CD":false,"RA":true,"RD":true,"TC":false,"Status":0,
	"Question":[{"name":"wealdtech.eth.","type":16}],
	"Answer":[
	{"name":"wealdtech.eth","type":16,"TTL":3600,"data":"dnslink=/ipns/www.wealdtech.eth"},
	{"name":"wealdtech.eth","type":16,"TTL":3600,"data":"contenthash=0xe501017000117777772e7765616c64746563682e657468"},
	{"name":"wealdtech.eth","type":16,"TTL":3600,"data":"a=0x4760cF82331ee520573bbB332106353587E7eC49"}
	]
  }
*/

type txtRecord struct {
	Data string `json:"data"`
}

type ethlinkResponse struct {
	Answer []txtRecord
}

func ResolveEthLink(ensdomain string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://eth.link/dns-query?type=TXT&name="+ensdomain, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("content-type", "application/dns-json")
	req.Header.Set("User-Agent", "zebu")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bad status code %d", resp.StatusCode)
	}
	return parsEthLink(resp.Body)
}

func parsEthLink(body io.ReadCloser) (string, error) {
	var elresp ethlinkResponse
	if err := json.NewDecoder(body).Decode(&elresp); err != nil {
		return "", err
	}
	for _, txt := range elresp.Answer {

		parts := strings.Split(strings.Trim(txt.Data, "\""), "=")
		if len(parts) == 2 && strings.EqualFold(parts[0], "contenthash") {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("no contenthash txt record found")
}

//https://github.com/cpacia/ens-lite seems ideally but go get fails
//maybe because  github.com/ethereum/go-ethereum/contracts/ens moved to
// use https://github.com/wealdtech/go-ens/tree/master/contracts instead

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
