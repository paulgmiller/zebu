Distributerd social media/content posting.
Evolved from this
https://paulgmiller.github.io/2021/02/07/Distributed-Twitter.html


## TODO 

## Basics
simple follow links

###Bandwidth

Running ipfs in the cloud is a expensive bandwidth wise 
Here's ipfs stats over 5 days. 

k exec zipfs-0  -- ipfs stats bw
Bandwidth
TotalIn: 107 GB
TotalOut: 109 GB
RateIn: 116 kB/s
RateOut: 102 kB/s

Probably related to number of peers and kadmala. 
k exec zipfs-0  -- ipfs swarm peers | wc -l
1323

connmgr.highwatermark and routing=dhtclient seem to be two options

https://github.com/ipfs/kubo/issues/3429
https://github.com/ipfs/kubo/issues/3065
https://github.com/libp2p/research/pull/4


### UI 
* Limit posts with option to see more
* fix date formatting
* better image galleries.
* https://galleriajs.github.io/themes/
* https://www.liwen.id.au/heg/

### Embed ipfs?
Or use infura / web3.storage for hosted version
https://github.com/ipfs/go-ds-s3


## scale ipfs
loadbalancer with different ports
optionally return imags through cloudflare ipfs gateway 


## move to new ipfs api
gets us timeouts for not found

### ENS  and dns integration
https://github.com/web3/web3.js/issues/2683
let them register after connect


### protect against spam. 
Look at addreses account balance integral  of period 


## run importer
nitter has rss feeds. Let people follow people on twitter that way? 
https://nitter.net/about

## Activity pub integration?

## e2e test. !!!
* Create a new account
* register a dns name
* follow johnwilkes.northbriton.net and some imported rss fee.d
 

## parallize fetching.

## what is the best homepage
* suggest people to follow? G
* show random posts? Dangerous?


## retweets and likes
accumulate several likes before signing?
