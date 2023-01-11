Distributerd social media/content posting.
Evolved from this
https://paulgmiller.github.io/2021/02/07/Distributed-Twitter.html


## TODO 

## Basics
allow to re-register
likes 
individual post url
retweets?

### Bandwidth

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

highwater mark took us to 20gig/day. lets see if dhtclient is better.

### UI 
* Limit posts with option to see more
* better image galleries.
* https://galleriajs.github.io/themes/
* https://www.liwen.id.au/heg/
* better buttons for moblie. Hell maybe a mobile app

### Embed ipfs?
Or use infura / web3.storage for hosted version
https://github.com/ipfs/go-ds-s3


## scale ipfs
loadbalancer with different ports
optionally return imags through cloudflare ipfs gateway 
look at cluster ipfs and hole punching 


### protect against spam. 
Look at addreses account balance integral  of period 

## run importer
let rss feeds get added through api
nitter has rss feeds. Let people follow people on twitter that way? 
https://nitter.net/about

## Activity pub integration?

## e2e test. !!!
* Create a new account
* register a dns name
* follow johnwilkes.northbriton.net and some imported rss feed
 

## parallize fetching.

## what is the best homepage
* suggest people to follow? G
* show random posts? Dangerous?


