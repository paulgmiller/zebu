Distributerd social media/content posting.


## People to follow.
Still need to implment ENS. (dns txt entries might work? )
paul miller k51qzi5uqu5dhy7ghgmb0tedml0hw453vqaqo7tt3pl1koprzfwkgpf0ag6icj
Evolved from this
https://paulgmiller.github.io/2021/02/07/Distributed-Twitter.html

## Releases
```
miller@millercloud:~/zebu$ go build -o bin/linux/zebu
pmiller@millercloud:~/zebu$ ipfs add bin/linux/zebu 
added QmPQa5XSJR3cXP2PaC4ux2L56gcdaV9KeuD3VH9byoP7rf zebu
pmiller@millercloud:~/zebu$ GOOS=windows go build -o bin/windows/zebu 
pmiller@millercloud:~/zebu$ ipfs add  bin/windows/zebu 
added QmVFev3MJosBY2YQHrv7N3nTa1f2yCSg1YKWQ226YHgiGQ zebu
```