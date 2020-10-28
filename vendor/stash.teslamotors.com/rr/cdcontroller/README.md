# cdcontroller

This repository contains the Charge/Discharge Controller (server) for
RoadRunner formation.

## Overview

C/D Controller (CDC) performs the following tasks.

- Acts as server in gRPC Client/Server relationship with Conductor (CND)
  - LoadOperation (PreparedForDelivery/Desired) at 63000 (ingress/decision point for C/D)
  
    Determines to which aisle a tray should be routed using either a round robin, fixture availability algorithm,
    or a combination of both (this algorithm is not finalized). Responds with aisle location via a PreparedForDelivery/Complete
    message.
    
  - LoadOperation (PreparedForDelivery/Desired) at 630X0 where X is the aisle number.
  
    Determine to which fixture a tray should be routed to within X aisle. Responds with fixture location via a
    PreparedForDelivery/Complete message. Reserves fixture on relevant tower controller.
    
  - LoadOperation (Placed/Desired) at 630X0-CC-LL where X is the aisle number, CC is the column number, and LL is the level number.
  
    Send recipe and tray information to relevant tower controller. Respond with LoadOperation (Placed/Complete) message.
        
- Acts as server in HTTP REST client/server relationship with tower controllers.
  - Listens for Unload Requests from tower controller when a tray has completed it's recipe in the fixture.
  
    Sends an UnloadOperation (Executed/Complete) message to CND to request unload of that tray from C/D.
    
    If the tray is a commission-self-test tray it will try to re-place the tray back into the aisle in a fixture
    that still needs commissioning. If no fixtures need commissioning CDC will close the tray's process step and
    unload it from the aisle.
    
- Provides a status and operations API for data visualization and system control purposes.
  - API documentation WIP

## Install Locally

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

## Deploy to Kubernetes Cluster
### Deploy
```shell script
kubectl apply -f configuration/kubernetes/deploycdc.yaml
```

### Edit Configuration
Enter the following command
```shell script
kubectl edit configmap cdcontroller -n=formation-cdcontroller
```

This will bring up Vim, make the desired changes and write-quit. The changes will take effect immediately. 

To simply view the current configuration
```shell script
kubectl describe configmap cdcontroller -n=formation-cdcontroller
```
or
```shell script
kubecutl get configmap cdcontroller -n=formation-cdcontroller -o yaml
```

### Force Restart
To force a restart simply delete the pod (kubernetes will immediately remake it).

Get the pod ID
```shell script
kubectl get pod -n=formation-cdcontroller
```
Delete the pod
```shell script
kubectl delete pod <pod-id> -n=formation-cdcontroller
```