# towercontroller

This repository contains the state machine for the RoadRunner charge/discharge
tower controller (not to be confused with the charge/discharge controller).

## Install

In the root of the repo run

```bash
docker-compose build
cp configuration/system/daemon.json /etc/docker
cp configuration/system/towercontroller.service /etc/systemd/system
systemctl daemon-reload
systemctl start towercontroller
systemctl enable towercontroller
# follow logrotate instructions in configuration/logrotate/README.md
```

## System-level Port Strategy

- localhost:13160/proto: protostream publisher/towercontroller listener
- localhost:13161/proto: towercontroller publisher/protostream listener
- 0.0.0.0:13163: towercontroller <-> C/D Controller port
  - /avail:               fixture availability endpoint (C/D Controller GET)
  - /load:                tray load endpoint (C/D Controller POST)
  - /broadcast:           broadcast request endpoint (C/D Controller POST)
  - /preparedForDelivery: reservation request endpoint (C/D Controller POST)
- 0.0.0.0:13173: towercontroller user API
  - /form_request:        POST form requests to FXR
  - /equipment_request:   POST equipment requests to FXR
  - /unreserve:           POST unreservations for FXR