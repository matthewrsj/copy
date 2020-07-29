# towerproto

This repository contains the protobuf file that holds all signals being streamed
between towercontroller and fxr controllers in a tower.

## Contributing

When contributing please take the following steps to make the update usable by both sides of the communication bridge
(firmware and software).

- Generate the go code for software (TODO: make this part of the build)
  `protoc --go_out=. tower.proto alerts.proto # must have protoc-gen-go installed`
  -  To use protoc downloaded from firmware build you can do: `~/opt/build_tools/protoc-3.11.2-linux-x86_64/bin/protoc --proto_path=. --go_out=. tower.proto alerts.proto`
- Bump the semver git tag following www.semver.org standards
