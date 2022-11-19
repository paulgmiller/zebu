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

## Related to ens l2
https://eips.ethereum.org/EIPS/eip-3668
https://discuss.ens.domains/t/layer-2-scaling-and-subdomains/6286/10
https://github.com/ensdomains/ens-contracts/tree/master/contracts/wrapper

## TODO 
### better image galleries.
https://galleriajs.github.io/themes/
https://www.liwen.id.au/heg/

### Embend ipfs or containerize

### ENS integration
https://github.com/web3/web3.js/issues/2683

### run it on a cluster. 

### protect against spam. 

### shove on fly.io or aks cluster.