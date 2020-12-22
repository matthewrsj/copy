package cdcontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type cellMapResp struct {
	Cells []CellData `json:"cells"`
}

// CellData contains the serial number, location in tray, and a flag indicating whether or not the locaiton is empty
type CellData struct {
	Position   string `json:"position"`
	Serial     string `json:"cell_serial"`
	IsEmpty    bool   `json:"is_empty"`
	StatusCode int    `json:"status_code"`
}

// CellStatusRequest contains the data to post cell statuses
type CellStatusRequest struct {
	EquipmentName string       `json:"equipment_name"`
	RecipeName    string       `json:"recipe_name"`
	RecipeVersion int          `json:"recipe_version"`
	Cells         []CellPFData `json:"cells"`
}

// CellPFData contains the pass/fail data of a given cell
type CellPFData struct {
	Serial string `json:"cell_serial"`
	Status string `json:"status"`
}

// NextFormationStep contains the recipe name, step and step type for a given tray
type NextFormationStep struct {
	Name     string `json:"name"`
	Step     string `json:"step"`
	StepType string `json:"step_type"`
}

// Status strings for cell PF data status
const (
	StatusPassed = "pass"
	StatusFailed = "fail"
)

// CellAPIClient holds several ease-of-use methods for communicating with the Cell API
type CellAPIClient struct {
	baseURL string
	eps     endpoints
}

type endpoints struct {
	cellMapFmt         string
	processStatusFmt   string
	nextProcessStepFmt string
	cellStatusFmt      string
	closeProcessFmt    string
}

const (
	// DefaultCellMapFmt is the default endpoint for the cell mapping. Format directive is for the tray serial.
	DefaultCellMapFmt = "/trays/%s/cells"
	// DefaultProcessStatusFmt is the default endpoint for posting the process status of a tray.
	// Format directives are for the tray serial, process step name, and status.
	DefaultProcessStatusFmt = "/trays/%s/%s/%s"
	// DefaultNextProcessStepFmt is the default endpoint for getting the next process step of a tray.
	// Format directives is for the tray serial.
	DefaultNextProcessStepFmt = "/trays/%s/formation"
	// DefaultCellStatusFmt is the default endpoint for posting cell status.
	DefaultCellStatusFmt = "/trays/%s/formation/cd/status"
	// DefaultCloseProcessFmt is the default endpoint for closing a process step
	DefaultCloseProcessFmt = "/trays/%s/formation/next"
)

// CellAPIOption function to set internal fields on the client
type CellAPIOption func(*CellAPIClient)

// WithCellMapFmtEndpoint returns an CellAPIOption to set the client cell map endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithCellMapFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.cellMapFmt = epf
	}
}

// WithProcessStatusFmtEndpoint returns an CellAPIOption to set the client process status endpoint.
// The epf argument is a format string that accepts the tray serial number, process step, and status.
func WithProcessStatusFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.processStatusFmt = epf
	}
}

// WithNextProcessStepFmtEndpoint returns an CellAPIOption to set the client next process step endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithNextProcessStepFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.nextProcessStepFmt = epf
	}
}

// WithCellStatusFmtEndpoint returns an CellAPIOption to set the client cell status endpoint.
func WithCellStatusFmtEndpoint(ep string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.cellStatusFmt = ep
	}
}

// WithCloseProcessFmtEndpoint returns an option to set the client close process endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithCloseProcessFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.closeProcessFmt = epf
	}
}

// NewCellAPIClient returns a pointer to a new CellAPIClient object configured with opts.
func NewCellAPIClient(baseURL string, opts ...CellAPIOption) *CellAPIClient {
	c := CellAPIClient{
		baseURL: baseURL,
		eps: endpoints{
			cellMapFmt:         DefaultCellMapFmt,
			processStatusFmt:   DefaultProcessStatusFmt,
			nextProcessStepFmt: DefaultNextProcessStepFmt,
			cellStatusFmt:      DefaultCellStatusFmt,
			closeProcessFmt:    DefaultCloseProcessFmt,
		},
	}

	for _, opt := range opts {
		opt(&c)
	}

	return &c
}

// CloseProcessStep closes the process step for the tray SN in the cell API
func (c *CellAPIClient) CloseProcessStep(sn, rcpeName string, version int) error {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.closeProcessFmt, sn))

	type request struct {
		RecipeName    string `json:"current_recipe_name"`
		RecipeVersion int    `json:"current_recipe_version"`
	}

	// split the recipe name on the sep for version and only take the first part
	// since split always returns at least a 1-elem list this will not panic, even if the
	// name does not contain the version
	b, err := json.Marshal(request{RecipeName: strings.Split(rcpeName, " - ")[0], RecipeVersion: version})
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

	// nolint:gosec // easier to construct the URL
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("POST response from %s NOT OK: %v; %s", url, resp.StatusCode, resp.Status)
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
			"POST response from %s NOT OK: %v; %s; %s; %s",
			url,
			resp.StatusCode,
			resp.Status,
			eResp.Error.Message,
			eResp.Error.Description,
		)
	}

	return nil
}

// GetCellMap fetches the cell map for the given tray sn
func (c *CellAPIClient) GetCellMap(sn string) (map[string]CellData, error) {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.cellMapFmt, sn))

	// nolint:gosec // easier to construct the URL
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
	cm := make(map[string]CellData, maxCells)

	for _, cell := range cmr.Cells {
		if cell.IsEmpty || cell.StatusCode != 0 {
			// no cell present or cell previously failed
			continue
		}

		cm[cell.Position] = cell
	}

	return cm, nil
}

// UpdateProcessStatus updates the cell API of the start or end of a recipe
func (c *CellAPIClient) UpdateProcessStatus(sn, fixture string, s TrayStatus) error {
	if !s.isValid() {
		return fmt.Errorf("status %s is not valid", s)
	}

	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.processStatusFmt, sn, fixture, s))

	// nolint:gosec // easier to construct the URL
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
			return fmt.Errorf("POST response from %s NOT OK: %v; %s", url, resp.StatusCode, resp.Status)
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
			"POST response from %s NOT OK: %v; %s; %s; %s",
			url,
			resp.StatusCode,
			resp.Status,
			eResp.Error.Message,
			eResp.Error.Description,
		)
	}

	return nil
}

// SetCellStatusesNoClose sets the pass/fail data of a list of cells but does not close the step
func (c *CellAPIClient) SetCellStatusesNoClose(tray, eqName, recipe string, ver int, cpf []CellPFData) error {
	return c.setCellStatusesWithCloseOption(tray, eqName, recipe, ver, cpf, false)
}

// SetCellStatuses sets the pass/fail data of a list of cells
func (c *CellAPIClient) SetCellStatuses(tray string, eqName, recipe string, ver int, cpf []CellPFData) error {
	return c.setCellStatusesWithCloseOption(tray, eqName, recipe, ver, cpf, true)
}

func (c *CellAPIClient) setCellStatusesWithCloseOption(tray, eqName, recipe string, ver int, cpf []CellPFData, closeStep bool) error {
	req := CellStatusRequest{
		EquipmentName: eqName,
		RecipeName:    recipe,
		RecipeVersion: ver,
		Cells:         cpf,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.cellStatusFmt, tray))

	if !closeStep {
		// default is to close the step unless complete query parameter is set to 0
		url += "?complete=0"
	}

	// nolint:gosec // easier to construct the URL
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	defer func() {
		if cerr := resp.Body.Close(); err == nil {
			err = cerr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// ignore error here, it's just for enhancing the error string
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("POST response NOT OK: %v; %v", resp.StatusCode, string(b))
	}

	return nil
}

// GetNextProcessStep returns the NextFormationStep for the tray SN
func (c *CellAPIClient) GetNextProcessStep(sn string) (NextFormationStep, error) {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.nextProcessStepFmt, sn))

	// nolint:gosec // easier to construct the URL
	resp, err := http.Get(url)
	if err != nil {
		return NextFormationStep{}, fmt.Errorf("POST to %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	rBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return NextFormationStep{}, fmt.Errorf("read response body from %s: %v", url, err)
	}

	var r NextFormationStep

	if err := json.Unmarshal(rBody, &r); err != nil {
		return NextFormationStep{}, fmt.Errorf("unmarshal response body from %s: %v", url, err)
	}

	if r.Name == "" {
		return NextFormationStep{}, fmt.Errorf("next process step not defined for tray %s", sn)
	}

	return r, nil
}

func urlJoin(base, endpoint string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
}
