FROM artifactory.teslamotors.com:2046/python:3.8-slim-buster

# change to bash shell
SHELL ["/bin/bash", "-c"]

# set HOME variable
ENV HOME /tmp

# update docker container's OS and install dependencies 
RUN apt-get update
RUN apt-get install -y wget git unzip

# clean up after apt operations
RUN rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    apt-get autoremove -y && \
    apt-get clean

# download and install go
RUN wget https://dl.google.com/go/go1.16.5.linux-amd64.tar.gz
RUN tar -C /tmp -xzf go1.16.5.linux-amd64.tar.gz

# add Go's environment variables
ENV GO111MODULE=on
ENV GOPATH=/tmp/go
ENV PATH=$PATH:$GOPATH/bin

# download and install protobuf compiler
RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v3.11.2/protoc-3.11.2-linux-x86_64.zip
RUN unzip protoc-3.11.2-linux-x86_64.zip -d /tmp/.local

# add Protobuf compiler's environment variables
ENV PATH=$PATH:/tmp/.local/bin

# downlond and install protoc-gen-go plugin for protoc executable
RUN go get google.golang.org/protobuf/cmd/protoc-gen-go@v1.25.0
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.25.0