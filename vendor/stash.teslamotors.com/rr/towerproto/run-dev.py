#!/usr/bin/env python3

import os
import sys
import subprocess

# similar to ./run-dev script in the main firmare repo, you can use this script
#   to execute FW builds within a container (on local machines or ci-dev machines)
#
# examples:
#   ./run-dev.py protoc --go_out=. tower.proto alerts.proto

# grab command arguments argv
command_str = " ".join(sys.argv[1:])
print(f"running command = [{command_str}]")

# only allow certain commands to prevent unexpected things from happening
allowed_commands = ["protoc", "/"]

# get options for cgroup_str
tesla_cgroup_bin = (
    subprocess.check_output(["which", "tesla-cgroup"]).decode("utf-8").strip()
)

# build cgroup_str by using tesla-cgroup binary (present on ci-dev machines)
cgroup_str = ""
if tesla_cgroup_bin:
    cgroup_str = (
        "--cgroup-parent="
        + subprocess.check_output([tesla_cgroup_bin]).decode("utf-8").strip()
    )

if any(filter(command_str.startswith, allowed_commands)):
    # build the container based on the Dockerfile which is in the same directory
    print("Building docker container...")
    os.system("docker build . %s" % cgroup_str)

    # run the container with the following options:
    # --user to ensure files created exist under your uid/gid (would otherwise be owned by root)
    # -w to use the workdir that has our files
    # -v to expose the current dir as a path in the container
    # %s to pass cgroup options in (ensures user RAM/CPU limits are followed on shared ci-dev machines)
    # -it to select the image we just built
    # %s to pass the command string in
    print("Running in docker container...")

    os.system(
        "docker run  --user $(id -u):$(id -g)  -w /towerproto -v ${PWD}:/towerproto %s -it $(docker build -q .) %s"
        % (cgroup_str, command_str)
    )
else:
    print("Error: Unsupported command...exiting", file=sys.stderr)
