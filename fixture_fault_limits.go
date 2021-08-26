package towercontroller

import (
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
)

// only allow a tray to fault 1 fixture before flushing it from the system (on the second fault)
// originally this maximum with 3 retries (4 faults) but this has shown to be too lenient. A poorly-made
// recipe can cause a huge percentage of fallout over the entire line. Limiting the retries to 1 allows
// issues to be caught earlier.
const _maxAllowedFixtureFaults = 1

func holdTrayIfFaultsExceedLimit(logger *zap.SugaredLogger, ca *cdcontroller.CellAPIClient, conf Configuration, mockCellAPI bool, tid string) {
	if fr, err := getFaultRecord(logger, conf, tid); err != nil {
		logger.Errorw("unable to retrieve fault record from C/D Controller", "error", err)
	} else if faultsExceedLimit(fr) {
		logger.Warnw("tray has faulted too many fixtures, placing a hold on the tray", "faulted_fixtures", fr.FixturesFaulted)
		if mockCellAPI {
			logger.Warn("cell API transactions mocked, not holding tray")
			return
		}

		if err := ca.HoldTray(tid); err != nil {
			logger.Errorw("unable to hold tray", "error", err)
			return
		}

		logger.Info("tray placed in hold state in cell API")
	}
}

func faultsExceedLimit(fr cdcontroller.FaultRecord) bool {
	return len(fr.FixturesFaulted) > _maxAllowedFixtureFaults
}
