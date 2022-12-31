package zebu

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ipfs/go-dnslink"
	"github.com/wealdtech/go-ens/v3"
)

//other options https://eth.link/ doesn't actually support direct address resolution?

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

//https://github.com/cpacia/ens-lite seems ideally but go get fails
//maybe because  github.com/ethereum/go-ethereum/contracts/ens moved to
// use https://github.com/wealdtech/go-ens/tree/master/contracts instead

var NoEthEndpoint = errors.New("ETHENDPOINT not defined")

func resolveEns(ensdomain string) (string, error) {
	//obviusly need a light client but
	ethendpoint := os.Getenv("ETHENDPOINT")
	if ethendpoint == "" {
		return "", NoEthEndpoint
	}

	client, err := ethclient.Dial(ethendpoint)
	if err != nil {
		return "", err
	}

	// Resolve a name to an address
	resolver, err := ens.NewResolver(client, ensdomain)
	if err != nil {
		return "", err
	}
	addr, err := resolver.Address()
	if err != nil {
		return "", err
	}
	/*hash, err := resolver.Contenthash()
	if err != nil {
		return "", err
	}
	readablehash, err := ens.ContenthashToString(hash)
	if err != nil {
		return "", err
	}
	log.Printf("Content hash of %s is %s\n", ensdomain, readablehash)*/
	return addr.String(), nil

}

func RegisterDNS(displayname, publicname string) (string, error) {
	//see if this is already
	//take this env var? turn off on non prod
	endpoint, ok := os.LookupEnv("REGISTERENDPOINT")
	if !ok {
		endpoint = "northbriton"
	}

	url := fmt.Sprintf("http://%s/reserve/%s", endpoint, displayname)
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(publicname))
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("got %d : %s", resp.StatusCode, string(body))
	}
	//get this out of body?
	return displayname + ".northbriton.net", nil
}

const legacyipnsprefix = "/ipns"

var DNSNotFound = errors.New("DNSNOTFOUND")

//looksup dnslionk subdomains  and pulls out /zebu or /ipfs (legacy) TXT records
func resolveDns(dnsname string) (string, error) {
	txts, err := net.LookupTXT("_dnslink." + dnsname)
	if err != nil {
		log.Printf("failed to find _dnslink." + dnsname)
		derr := err.(*net.DNSError)
		if derr.IsNotFound {
			return "", DNSNotFound
		}
		return "", err
	}
	link := ""
	for _, t := range txts {
		link, err = dnslink.ParseTXT(t)
		if err == nil {
			continue
		}
		log.Printf("invalid dns link %s", t)
	}

	if link == "" {
		return "", DNSNotFound
	}

	if strings.HasPrefix(link, centraltopic) {
		return link[len(centraltopic)+1:], nil
	}
	//remove after we swith over.
	if strings.HasPrefix(link, legacyipnsprefix) {
		return link[len(legacyipnsprefix)+1:], nil
	}
	log.Printf("link %s didn't match prefixes", link)
	return "", DNSNotFound
}

//resolves tries a couple strategy to resolve a user to an eth acccount
func Resolve(id string) (string, error) {

	//nothing to do here.
	if strings.HasPrefix(id, "0x") {
		return id, nil
	}

	//add some caching?
	if strings.HasSuffix(id, ".eth") {
		return resolveEns(id)
	}
	//fall back to dns last.
	return resolveDns(id)
}
