FROM golang:stretch

WORKDIR /go/src/app
COPY . .

RUN cd cmd && go build -mod vendor -o tcapp . && cd ..
RUN chmod +x cmd/tcapp

CMD cd cmd && ./tcapp
