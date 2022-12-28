TAG=$(git rev-parse --short HEAD)
docker build . -t paulgmiller/zebu:$TAG
docker push paulgmiller/zebu:$TAG