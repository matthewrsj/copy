# cdcontroller

This repository contains the Charge/Discharge Controller (server) for
RoadRunner formation.

## Install

```shell script
docker-compose build                                   # build the docker container
mkdir /etc/cdcontroller.d                              # make the configuration directory
cp configuration/server/server.yaml /etc/cdcontroller.d              # copy the configuration into place
cp configuration/system/cdcontroller.service /etc/systemd/system     # copy the service file into place
systemctl daemon-reload                                # tell systemd to reload service files from disk
systemctl start cdcontroller                           # start the cdcontroller service
systemctl enable cdcontroller                          # enable the cdcontroller service to start on boot
systemctl status cdcontroller                          # confirm the service is running
# see configuration/logrotate/README.md for instructions to set up logrotate
```

Logs can be found (by default) at `/var/log/cdcontroller/server.log`.


By default the C/D Controller gRPC server will be running at port 13175 and the
HTTP server for Tower Controllers to talk to will be running at port 8080.
