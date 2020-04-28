package main

import (
	"stash.teslamotors.com/ctet/statemachine"
	"stash.teslamotors.com/rr/towercontroller"
)

func main() {
	statemachine.RunFrom(&towercontroller.TrayBarcode{})
}
