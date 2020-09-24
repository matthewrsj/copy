FROM golang:stretch

WORKDIR /go/src/app
COPY . .

# install root certificate
RUN wget http://pki.tesla.com/pki/SJC04P1ADCRC01_Corporate%20Root%20CA.crt -O- | openssl x509 -inform der -out /usr/local/share/ca-certificates/tesla_root_ca.crt \
    && update-ca-certificates

RUN cd cmd && go build -mod vendor -o cdcapp . && cd ..
RUN chmod +x cmd/cdcapp
RUN mkdir /var/log/cdcontroller

CMD cd cmd && ./cdcapp -loglvl debug -logf /var/log/cdcontroller/server.log -conf ../configuration/server/server.yaml