FROM golang:1.19

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app ./...

CMD ["app"]

# build image:
# docker build -t dockersync .

# run container
#docker run --rm -d -p 3333:3333 -v /var/run/docker.sock:/var/run/docker.sock \
#    -e ENV_REPO="redis" \
#    -e ENV_TAG=":latest" \
#    -e ENV_SERVICE_ID="mw682hmq9o2a" \
#    -e ENV_WEBHOOK_TOKEN="BbHw3mvGZBUCsKE2XL0ck4F9bHt0jz4g72Nm55aPq7DJnzr80W" \
#    -e ENV_SERVICESYNC_PORT="3333" \
#    dockersync