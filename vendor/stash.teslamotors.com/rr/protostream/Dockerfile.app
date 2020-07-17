FROM golang:stretch

WORKDIR /go/src/app
COPY . .

RUN cd cmd && go build -mod vendor -o protostream . && cd ..
RUN chmod +x cmd/protostream

CMD cd cmd && ./protostream -loglvl debug
