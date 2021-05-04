FROM golang:stretch

WORKDIR /go/src/app
COPY . .

RUN apt update -y && apt upgrade -y && apt install -y bash git build-essential
RUN cd / && go get github.com/ahmetb/govvv
RUN cd cmd && govvv build -mod vendor -o tcapp .
RUN chmod +x cmd/tcapp

CMD cd cmd && ./tcapp
