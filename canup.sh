#!/bin/bash
modprobe can
modprobe vcan
modprobe slcan
ip link set can0 up
ip link set can0 up type can bitrate 500000 dbitrate 2000000 dsample-point 0.75 fd on
ip link set can0 txqueuelen 1000
ip link set can1 up
ip link set can1 up type can bitrate 500000 dbitrate 2000000 dsample-point 0.75 fd on
ip link set can1 txqueuelen 1000
pushd /opt/canup/can-isotp
make clean
make
make modules_install
insmod ./net/can/can-isotp.ko 

