//+build !test

package main

import (
	"log"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/towercontroller"
)

func handleManualOperation(s *statemachine.Scheduler, logger *zap.SugaredLogger, caClient *cellapi.Client, mockCellAPI bool) {
	logger.Info("waiting for tray barcode scan")

	for {
		barcodes, err := towercontroller.ScanBarcodes(caClient, mockCellAPI, logger)
		if err != nil {
			if towercontroller.IsInterrupt(err) {
				log.Fatal("received CTRL-C, exiting...")
			}

			logger.Warnw("scan barcodes", "error", err)

			continue
		}

		barcodes.ManualMode = true // handle some internal ops differently (not commanded via network)
		barcodes.MockCellAPI = mockCellAPI

		if err := s.Schedule(statemachine.Job{Name: towercontroller.IDFromFXR(barcodes.Fixture), Work: barcodes}); err != nil {
			logger.Warnw("schedule tray on fixture", "error", err)
			continue
		}

		logger.Infow("state machine started on tray", "tray", barcodes.Tray.Raw, "fixture", barcodes.Tray.Raw)
	}
}
