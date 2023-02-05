FROM ipfs/kubo:v0.17.0
#kube 0.18 is availabe but needs a migration https://github.com/ipfs/fs-repo-migrations/blob/master/run.md

ADD ipfs-config.sh /container-init.d/ipfs-config.sh
RUN chmod a+x /container-init.d/ipfs-config.sh

EXPOSE 4001/udp
EXPOSE 4001/tcp