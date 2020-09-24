package towercontroller

import (
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
