package cdcontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const _pfdEndpoint = "/preparedForDelivery"

func reserveOnTower(tower *Tower, tray string, fxrID string) error {
	pfd := PreparedForDelivery{
		Tray:    tray,
		Fixture: fxrID,
	}

	buf, err := json.Marshal(pfd)
	if err != nil {
		return fmt.Errorf("json Marshal: %v", err)
	}

	resp, err := http.Post(tower.Remote+_pfdEndpoint, "application/json", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("post: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post response NOT OK: %d; %s", resp.StatusCode, resp.Status)
	}

	return nil
}
