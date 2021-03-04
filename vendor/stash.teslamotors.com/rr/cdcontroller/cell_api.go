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

// CellData contains the serial number, location in tray, and a flag indicating whether or not the location is empty
type CellData struct {
	Position   string `json:"position"`
	Serial     string `json:"cell_serial"`
	IsEmpty    bool   `json:"is_empty"`
	StatusCode int    `json:"status_code"`
}

// CellStatusRequest contains the data to post cell statuses
type CellStatusRequest struct {
	EquipmentName string            `json:"equipment_name"`
	CellStatus    map[string]string `json:"status"`
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
	trayHoldFmt        string
	trayReleaseFmt     string
}

const (
	// DefaultCellMapFmt is the default endpoint for the cell mapping. Format directive is for the tray serial.
	DefaultCellMapFmt = "/trays/%s/cells"
	// DefaultProcessStatusFmt is the default endpoint for posting the process status of a tray.
	// Format directives are for the tray serial, start/end, process step name, and recipe version.
	DefaultProcessStatusFmt = "/trays/%s/cd/%s/%s/%d"
	// DefaultNextProcessStepFmt is the default endpoint for getting the next process step of a tray.
	// Format directives is for the tray serial.
	DefaultNextProcessStepFmt = "/trays/%s/formation"
	// DefaultCellStatusFmt is the default endpoint for posting cell status.
	DefaultCellStatusFmt = "/trays/%s/formation/cd/status"
	// DefaultCloseProcessFmt is the default endpoint for closing a process step
	DefaultCloseProcessFmt = "/trays/%s/formation/next"
	// DefaultTrayHoldFmt is default endpoint for flushing a tray to inspection station
	DefaultTrayHoldFmt = "/trays/%s/hold"
	// DefaultTrayReleaseFmt is the default endpoint for releasing a held tray
	DefaultTrayReleaseFmt = "/trays/%s/release"
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

// WithTrayHoldFmtEndpoint returns an option to set the client tray hold endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithTrayHoldFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.trayHoldFmt = epf
	}
}

// WithTrayReleaseFmtEndpoint returns an option to set the client tray release endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithTrayReleaseFmtEndpoint(epf string) CellAPIOption {
	return func(c *CellAPIClient) {
		c.eps.trayReleaseFmt = epf
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
			trayHoldFmt:        DefaultTrayHoldFmt,
			trayReleaseFmt:     DefaultTrayReleaseFmt,
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

// GetCellMapForCommissioning ignores previously failed locations when generating the returned map
// to allow commissioning trays to continue testing locations
func (c *CellAPIClient) GetCellMapForCommissioning(sn string) (map[string]CellData, error) {
	return c.getCellMapWithStatusOption(sn, false)
}

// GetCellMap fetches the cell map for the given tray sn
func (c *CellAPIClient) GetCellMap(sn string) (map[string]CellData, error) {
	return c.getCellMapWithStatusOption(sn, true)
}

func (c *CellAPIClient) getCellMapWithStatusOption(sn string, useStatus bool) (map[string]CellData, error) {
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
		if cell.IsEmpty || (useStatus && cell.StatusCode != 0) {
			// no cell present or cell previously failed
			continue
		}

		cm[cell.Position] = cell
	}

	return cm, nil
}

// StartProcess updates the cell API of the start or end of a recipe
func (c *CellAPIClient) StartProcess(sn, fixture, recipe string, version int) error {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.processStatusFmt, sn, "start", recipe, version))

	eq := struct {
		EqName string `json:"equipment_name"`
	}{
		EqName: fixture,
	}

	jb, err := json.Marshal(eq)
	if err != nil {
		return fmt.Errorf("marshal request: %v", err)
	}

	// nolint:gosec // easier to construct the URL
	resp, err := http.Post(url, "application/json", bytes.NewReader(jb))
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
func (c *CellAPIClient) SetCellStatusesNoClose(tray, eqName, recipe string, ver int, cpf map[string]string) error {
	return c.setCellStatusesWithCloseOption(tray, eqName, recipe, ver, cpf, false)
}

// SetCellStatuses sets the pass/fail data of a list of cells
func (c *CellAPIClient) SetCellStatuses(tray string, eqName, recipe string, ver int, cpf map[string]string) error {
	return c.setCellStatusesWithCloseOption(tray, eqName, recipe, ver, cpf, true)
}

func (c *CellAPIClient) setCellStatusesWithCloseOption(tray, eqName, recipe string, ver int, cpf map[string]string, closeStep bool) error {
	req := CellStatusRequest{
		EquipmentName: eqName,
		CellStatus:    cpf,
	}

	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.processStatusFmt, tray, "end", recipe, ver))

	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

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

// GetStepConfiguration returns the step configuration for the tray
func (c *CellAPIClient) GetStepConfiguration(sn string) (StepConfiguration, error) {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.nextProcessStepFmt, sn))

	// nolint:gosec // easier to construct the URL
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	rBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body from %s: %v", url, err)
	}

	respData := struct {
		StepConfig map[string]Step `json:"step_configuration"`
	}{}

	if err := json.Unmarshal(rBody, &respData); err != nil {
		return nil, fmt.Errorf("unmarshal response body from %s: %v", url, err)
	}

	return NewStepConfiguration(respData.StepConfig)
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

type holdRequest struct {
	EquipmentName string `json:"equipment_name"`
	Reason        string `json:"reason"`
}

// HoldTray puts a hold on the tray and sends it to the inspection station
func (c *CellAPIClient) HoldTray(sn string) error {
	const (
		holdEquip  = "test-hold"
		holdReason = "flush"
	)

	hr := holdRequest{
		EquipmentName: holdEquip,
		Reason:        holdReason,
	}

	hrb, err := json.Marshal(hr)
	if err != nil {
		return fmt.Errorf("marshal request: %v", err)
	}

	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.trayHoldFmt, sn))

	resp, err := http.Post(url, "application/json", bytes.NewReader(hrb))
	if err != nil {
		return fmt.Errorf("post request: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post request status code NOT OK: status_code %d, status %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// ReleaseTray releases a held tray to return to regular step
func (c *CellAPIClient) ReleaseTray(sn string) error {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.trayReleaseFmt, sn))

	resp, err := http.Post(url, "", bytes.NewReader([]byte{}))
	if err != nil {
		return fmt.Errorf("post request: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post request status code NOT OK: status_code %d, status %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func urlJoin(base, endpoint string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
}
