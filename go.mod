module stash.teslamotors.com/rr/towercontroller

go 1.14

require (
	bou.ke/monkey v1.0.2
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/manifoldco/promptui v0.7.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.3.0
	nanomsg.org/go/mangos/v2 v2.0.8
	stash.teslamotors.com/cas/asrs v0.0.0-20200623062440-9cee2d665dfb
	stash.teslamotors.com/ctet/go-socketcan v0.0.2
	stash.teslamotors.com/ctet/statemachine/v2 v2.0.1
	stash.teslamotors.com/rr/cellapi v0.0.3
	stash.teslamotors.com/rr/protostream v0.1.3
	stash.teslamotors.com/rr/towerproto v0.0.13
	stash.teslamotors.com/rr/traycontrollers v0.1.3
)

replace stash.teslamotors.com/rr/traycontrollers v0.1.3 => /home/parallels/projects/traycontrollers
