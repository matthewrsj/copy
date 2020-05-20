FROM golang:alpine

# install golangci-lint
RUN wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.27.0

# install git for go get
RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh

# install gocov
RUN go get github.com/axw/gocov/gocov

COPY . .

