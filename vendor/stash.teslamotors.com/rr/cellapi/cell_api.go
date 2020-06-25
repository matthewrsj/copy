package cellapi

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

type CellData struct {
	Position string `json:"position"`
	Serial   string `json:"cell_serial"`
	IsEmpty  bool   `json:"is_empty"`
}

type CellPFDataSWIFT struct {
	Serial  string `json:"cell_serial"`
	Status  string `json:"status"`
	Process string `json:"process"`
}

type CellPFData struct {
	Serial  string `json:"cell_serial"`
	Status  string `json:"status"`
	Recipe  string `json:"recipe"`
	Version int    `json:"version"`
}

// Status strings for cell PF data status
const (
	StatusPassed = "pass"
	StatusFailed = "fail"
)

type Client struct {
	baseURL string
	eps     endpoints
}

type endpoints struct {
	cellMapFmt         string
	processStatusFmt   string
	nextProcessStepFmt string
	cellStatus         string
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
	// DefaultCellStatus is the default endpoint for posting cell status.
	DefaultCellStatus = "/cells/set_cell_status"
)

// Option function to set internal fields on the client
type Option func(*Client)

// WithCellMapFmtEndpoint returns an Option to set the client cell map endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithCellMapFmtEndpoint(epf string) Option {
	return func(c *Client) {
		c.eps.cellMapFmt = epf
	}
}

// WithProcessStatusFmtEndpoint returns an Option to set the client process status endpoint.
// The epf argument is a format string that accepts the tray serial number, process step, and status.
func WithProcessStatusFmtEndpoint(epf string) Option {
	return func(c *Client) {
		c.eps.processStatusFmt = epf
	}
}

// WithNextProcessStepFmtEndpoint returns an Option to set the client next process step endpoint.
// The epf argument is a format string that accepts the tray serial number.
func WithNextProcessStepFmtEndpoint(epf string) Option {
	return func(c *Client) {
		c.eps.nextProcessStepFmt = epf
	}
}

// WithCellStatusEndpoint returns an Option to set the client cell status endpoint.
func WithCellStatusEndpoint(ep string) Option {
	return func(c *Client) {
		c.eps.cellStatus = ep
	}
}

// NewClient returns a pointer to a new Client object configured with opts.
func NewClient(baseURL string, opts ...Option) *Client {
	c := Client{
		baseURL: baseURL,
		eps: endpoints{
			cellMapFmt:         DefaultCellMapFmt,
			processStatusFmt:   DefaultProcessStatusFmt,
			nextProcessStepFmt: DefaultNextProcessStepFmt,
			cellStatus:         DefaultCellStatus,
		},
	}

	for _, opt := range opts {
		opt(&c)
	}

	return &c
}

func (c *Client) GetCellMap(sn string) (map[string]CellData, error) {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.cellMapFmt, sn))

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
	cm := make(map[string]CellData, maxCells)

	for _, cell := range cmr.Cells {
		if cell.IsEmpty {
			continue
		}

		cm[cell.Position] = cell
	}

	return cm, nil
}

func (c *Client) UpdateProcessStatus(sn, rcpeName string, s TrayStatus) error {
	if !s.isValid() {
		return fmt.Errorf("status %s is not valid", s)
	}

	process, err := RecipeToProcess(rcpeName)
	if err != nil {
		return fmt.Errorf("determine process name: %v", err)
	}

	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.processStatusFmt, sn, process, s))

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

func (c *Client) SetCellStatuses(cpf []CellPFData) error {
	type request struct {
		Cells []CellPFData `json:"cells"`
	}

	b, err := json.Marshal(request{Cells: cpf})
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

	url := urlJoin(c.baseURL, c.eps.cellStatus)

	// of course the URL has to be variable. We need to construct it.
	// nolint:gosec
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// ignore error here, it's just for enhancing the error string
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("POST response NOT OK: %v; %v", resp.StatusCode, string(b))
	}

	return nil
}

func (c *Client) SetCellStatusesSWIFT(cpf []CellPFDataSWIFT) error {
	type request struct {
		Cells []CellPFDataSWIFT `json:"cells"`
	}

	b, err := json.Marshal(request{Cells: cpf})
	if err != nil {
		return fmt.Errorf("marshal request json: %v", err)
	}

	url := urlJoin(c.baseURL, c.eps.cellStatus)

	// of course the URL has to be variable. We need to construct it.
	// nolint:gosec
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST to %s: %v", url, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// ignore error here, it's just for enhancing the error string
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("POST response NOT OK: %v; %v", resp.StatusCode, string(b))
	}

	return nil
}

func (c *Client) GetNextProcessStep(sn string) (string, error) {
	url := urlJoin(c.baseURL, fmt.Sprintf(c.eps.nextProcessStepFmt, sn))

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

	r.Next = strings.ToUpper(r.Next)

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

	return "", fmt.Errorf("invalid process step %s defined for this tray %s", r.Next, sn)
}

func urlJoin(base, endpoint string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
}
