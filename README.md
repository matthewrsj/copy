# towercontroller

This repository contains the state machine for the RoadRunner charge/discharge
tower controller (not to be confused with the charge/discharge controller).

## Install

In the root of the repo run

```bash
docker-compose build
docker-compose run towercontroller
```

## System-level Port Strategy

- localhost:13160/proto: protostream publisher/towercontroller listener
- localhost:13161/proto: towercontroller publisher/protostream listener
- 0.0.0.0:13163: towercontroller <-> C/D Controller port
  - /avail: fixture availability endpoint (C/D Controller GET)
  - /load:  tray load endpoint (C/D Controller POST)