package towercontroller

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
)

const (
	_testCMRespBody = `{
	"tray_serial": "00000000",
	"cells": [
		{
			"position": "A01",
			"is_empty": false,
			"cell_serial": "TESTA1"
		},
		{
			"position": "A02",
			"is_empty": false,
			"cell_serial": "TESTA2"
		},
		{
			"position": "A03",
			"is_empty": true,
			"cell_serial": ""
		}
	]
}`
	_testProcRespBody = `{
	"cells": {
		"TEST01": {
			"passed": true,
			"process": [
				[
					"OCV_ASSEMBLY",
					"2020-04-04 12:56:32",
					"0.15V / 5.0mOhm",
					false
				],
				[
					"QUALITY_CYCLING",
					"2020-05-04 12:46:05",
					"START",
					true
				]
			],
			"current": [
				"QUALITY_CYCLING",
				"START"
			],
			"position": "A01"
		}
	},
	"next": "QUALITY_CYCLING/END"
}
`
)

func httpGetPatch(body string) func(string) (*http.Response, error) {
	return func(string) (*http.Response, error) {
		respBody := ioutil.NopCloser(strings.NewReader(body))

		return &http.Response{
			Status:           "OK",
			StatusCode:       200,
			Body:             respBody,
			ContentLength:    int64(len(body)),
			TransferEncoding: []string{"encoding/json"},
		}, nil
	}
}

func Test_getCellMap(t *testing.T) {
	// monkey.Patch is confusing linter
	// nolint:bodyclose
	p := monkey.Patch(http.Get, httpGetPatch(_testCMRespBody))
	defer p.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			CellMapFmt: "/trays/%s/cells",
		},
	}

	v, err := getCellMap(c, "00000000")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2 /* expected */, len(v), "expect len to match")

	sn, ok := v["A01"]
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, "TESTA1" /* expected */, sn, "expect SN to match")

	sn2, ok := v["A02"]
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, "TESTA2" /* expected */, sn2, "expect SN to match")
}

func Test_getCellMapError(t *testing.T) {
	p := monkey.Patch(http.Get, func(string) (*http.Response, error) {
		return nil, assert.AnError
	})

	defer p.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			CellMapFmt: "/trays/%s/cells",
		},
	}

	_, err := getCellMap(c, "00000000")
	assert.Equal(t, true /* expected */, err != nil, "expect non-nil error")
}

func Test_getCellMapBodyReadErr(t *testing.T) {
	rPatch := monkey.Patch(ioutil.ReadAll, func(io.Reader) ([]byte, error) {
		return nil, assert.AnError
	})

	// monkey.Patch is confusing linter
	// nolint:bodyclose
	gPatch := monkey.Patch(http.Get, httpGetPatch(_testCMRespBody))

	defer rPatch.Unpatch()
	defer gPatch.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			CellMapFmt: "/trays/%s/cells",
		},
	}

	_, err := getCellMap(c, "00000000")
	assert.Equal(t, true /* expected */, err != nil, "expect non-nil error")
}

func Test_getCellMapUnmarshalErr(t *testing.T) {
	// monkey.Patch is confusing linter
	// nolint:bodyclose
	gPatch := monkey.Patch(http.Get, httpGetPatch(_testCMRespBody))
	jPatch := monkey.Patch(json.Unmarshal, func([]byte, interface{}) error {
		return assert.AnError
	})

	defer jPatch.Unpatch()
	defer gPatch.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			CellMapFmt: "/trays/%s/cells",
		},
	}

	_, err := getCellMap(c, "0000000")
	assert.Equal(t, true /* expected */, err != nil, "expect non-nil error")
}

func Test_updateProcessStatus(t *testing.T) {
	var calledWith string

	p := monkey.Patch(http.Post, func(url string, t string, r io.Reader) (*http.Response, error) {
		calledWith = url

		return &http.Response{
			Status:     "OK",
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})
	defer p.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			ProcessStatusFmt: "/trays/%s/%s/%s",
		},
	}

	if err := updateProcessStatus(c, "00000000", "FORM_PRECHARGE", _statusStart); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "https://base.url/module/api/v0.1/trays/00000000/precharge/start", calledWith)
}

func Test_updateProcessStatusInvalidStatus(t *testing.T) {
	err := updateProcessStatus(cellAPIConf{}, "00000000", "FORM_PRECHARGE", 0)
	assert.EqualError(t, err, "status 0 is not valid")
}

func Test_updateProcessStatusInvalidRecipe(t *testing.T) {
	err := updateProcessStatus(cellAPIConf{}, "00000000", "NOTHING", _statusEnd)
	assert.EqualError(t, err, "determine process name: step NOTHING is not valid")
}

func Test_setCellStatuses(t *testing.T) {
	var calledWith, body string

	p := monkey.Patch(http.Post, func(url string, t string, r io.Reader) (*http.Response, error) {
		calledWith = url
		b := make([]byte, 256)
		_, _ = r.Read(b)
		body = strings.Trim(string(b), "\x00 ")

		return &http.Response{
			Status:     "OK",
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})

	defer p.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			CellStatus: "/cells/set_cell_status",
		},
	}

	cpf := []cellPFData{
		{
			Serial:  "TESTA1",
			Process: "precharge",
			Status:  "pass",
		},
		{
			Serial:  "TESTA2",
			Process: "precharge",
			Status:  "fail",
		},
	}

	if err := setCellStatuses(c, cpf); err != nil {
		t.Fatal(err)
	}

	assert.Equal(
		t,
		"https://base.url/module/api/v0.1/cells/set_cell_status",
		calledWith,
		"expect URL to be constructed properly",
	)
	assert.Equal(
		t,
		`{"cells":[{"cell_serial":"TESTA1","process":"precharge","status":"pass"},`+
			`{"cell_serial":"TESTA2","process":"precharge","status":"fail"}]}`,
		body,
		"expect body to be constructed correctly",
	)
}

func Test_getNextProcessStep(t *testing.T) {
	// monkey.Patch is confusing linter
	// nolint:bodyclose
	p := monkey.Patch(http.Get, httpGetPatch(_testProcRespBody))
	defer p.Unpatch()

	c := cellAPIConf{
		Base: "https://base.url/module/api/v0.1",
		Endpoints: endpointConf{
			NextProcStepFmt: "/trays/%s/formation",
		},
	}

	step, err := getNextProcessStep(c, "00000000")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "FORM_CYCLE", step, "expect process step to be correct")
}
