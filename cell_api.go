package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type cellAPIConf struct {
	Base      string       `yaml:"base"`
	Endpoints endpointConf `yaml:"endpoints"`
}

// endpointConf contains the various endpoints used to communicate with the cell API
// Members with the `Fmt` suffix have format directives embedded for various options.
type endpointConf struct {
	// fmt: tray_serial
	CellMapFmt string `yaml:"cell_map"`
	// fmt: tray_serial|process|status
	ProcessStatusFmt string `yaml:"process_status"`
	// fmt: tray_serial
	NextProcStepFmt string `yaml:"next_process_step"`

	// no format here
	CellStatus string `yaml:"cell_status"`
}

type cellMapResp struct {
	Cells []cellData `json:"cells"`
}

type cellData struct {
	Position string `json:"position"`
	Serial   string `json:"cell_serial"`
	IsEmpty  bool   `json:"is_empty"`
}

type cellPFData struct {
	Serial  string `json:"cell_serial"`
	Process string `json:"process"`
	Status  string `json:"status"`
}

func getCellMap(apiConf cellAPIConf, sn string) (map[string]cellData, error) {
	url := urlJoin(apiConf.Base, fmt.Sprintf(apiConf.Endpoints.CellMapFmt, sn))

	// of course the URL has to be variable. We need to fmt in the tray_serial.
	// nolint:gosec
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http GET %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST response NOT OK: %v; %s", resp.StatusCode, resp.Status)
	}

	rBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body from %s: %v", url, err)
	}

	var cmr cellMapResp
	if err := json.Unmarshal(rBody, &cmr); err != nil {
		return nil, fmt.Errorf("unmarshal cell map response from %s: %v", url, err)
	}

	const maxCells = 128 // 128 is maximum number of cells in a tray
	cm := make(map[string]cellData, maxCells)

	for _, cell := range cmr.Cells {
		if cell.IsEmpty {
			continue
		}

		cm[cell.Position] = cell
	}

	return cm, nil
}

func updateProcessStatus(apiConf cellAPIConf, sn, rcpeName string, s status) error {
	if !s.isValid() {
		return fmt.Errorf("status %s is not valid", s)
	}

	process, err := recipeToProcess(rcpeName)
	if err != nil {
		return fmt.Errorf("determine process name: %v", err)
	}

	url := urlJoin(apiConf.Base, fmt.Sprintf(apiConf.Endpoints.ProcessStatusFmt, sn, process, s))

	// of course the URL has to be variable. We need to fmt everything in.
	// nolint:gosec
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("POST response NOT OK: %v; %s", resp.StatusCode, resp.Status)
		}

		type errMsg struct {
			Message     string `json:"message"`
			Description string `json:"description"`
		}

		type response struct {
			Error errMsg `json:"error"`
		}

		var eResp response
		if err := json.Unmarshal(body, &eResp); err != nil {
			return fmt.Errorf("POST response NOT OK: %v; %s", resp.StatusCode, resp.Status)
		}

		return fmt.Errorf(
			"POST response NOT OK: %v; %s; %s; %s",
			resp.StatusCode,
			resp.Status,
			eResp.Error.Message,
			eResp.Error.Description,
		)
	}

	return nil
}

func setCellStatuses(apiConf cellAPIConf, cpf []cellPFData) error {
	type request struct {
		Cells []cellPFData `json:"cells"`
	}

	b, err := json.Marshal(request{Cells: cpf})
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

	url := urlJoin(apiConf.Base, apiConf.Endpoints.CellStatus)

	// of course the URL has to be variable. We need to construct it.
	// nolint:gosec
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST response NOT OK: %v", resp.StatusCode)
	}

	return nil
}

func getNextProcessStep(apiConf cellAPIConf, sn string) (string, error) {
	url := urlJoin(apiConf.Base, fmt.Sprintf(apiConf.Endpoints.NextProcStepFmt, sn))

	// of course the URL has to be variable. We need to construct it.
	// nolint:gosec
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("POST to %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	rBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body from %s: %v", url, err)
	}

	r := struct {
		Next string `json:"next"`
	}{}

	if err := json.Unmarshal(rBody, &r); err != nil {
		return "", fmt.Errorf("unmarshal response body from %s: %v", url, err)
	}

	if r.Next == "" {
		return "", fmt.Errorf("next process step not defined for tray %s", sn)
	}

	prefixes := map[string]string{
		"PRECHARGE":       "FORM_PRECHARGE",
		"FIRST_CHARGE":    "FORM_FIRST_CHARGE",
		"FINAL_CD":        "FORM_SECOND_CHARGE",
		"QUALITY_CYCLING": "FORM_CYCLE",
	}

	for prefix, step := range prefixes {
		if strings.HasPrefix(r.Next, prefix) {
			return step, nil
		}
	}

	return "", fmt.Errorf("invalid process step %s defined for this tray", r.Next)
}

func urlJoin(base, endpoint string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
}
