package towercontroller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
)

func getCellMap(mockCellAPI bool, logger *zap.SugaredLogger, ca *cdcontroller.CellAPIClient, tray string) (map[string]cdcontroller.CellData, error) {
	if !mockCellAPI {
		logger.Info("GetCellMap")
		return ca.GetCellMap(tray)
	}

	logger.Warn("cell API mocked, skipping GetCellMap and populating a few cells")

	return map[string]cdcontroller.CellData{
		"A01": {
			Position: "A01",
			Serial:   "TESTA01",
			IsEmpty:  false,
		},
		"A02": {
			Position: "A02",
			Serial:   "TESTA02",
			IsEmpty:  false,
		},
		"A03": {
			Position: "A03",
			Serial:   "TESTA03",
			IsEmpty:  false,
		},
		"A04": {
			Position: "A04",
			Serial:   "TESTA04",
			IsEmpty:  false,
		},
	}, nil
}

func getRecipeVersion(mockCellAPI bool, logger *zap.SugaredLogger, ca *cdcontroller.CellAPIClient, tray string) int {
	if mockCellAPI {
		return 1
	}

	var recipeVersion int

	bo := backoff.NewExponentialBackOff()
	bo.MaxInterval = time.Minute
	bo.MaxElapsedTime = 0 // try forever

	// will never return a perm error
	_ = backoff.Retry(func() error {
		fs, err := ca.GetNextProcessStep(tray)
		if err != nil {
			logger.Errorw("get next process step", zap.Error(err))
			return err
		}

		fields := strings.Split(fs.Step, " - ")
		if len(fields) != 2 {
			logger.Errorw("invalid step from cell API", "step", fs.Step)
			return fmt.Errorf("invalid step from cell API: '%s'", fs.Step)
		}

		if recipeVersion, err = strconv.Atoi(fields[1]); err != nil {
			logger.Errorw("invalid step from cell API, unable to convert to int", "step", fs.Step, "error", err)
			return fmt.Errorf("invalid step from cell API, unable to convert to int: '%s'; %v", fs.Step, err)
		}

		return nil
	}, bo)

	logger.Infow("recipe version retrieved from cell API", "version", recipeVersion)

	return recipeVersion
}
