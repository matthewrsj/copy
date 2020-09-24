package cdcontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
)

// Tower contains the state of the towers in the system as well as the remote address with which to communicate
type Tower struct {
	FXRs   *FXRLayout
	Remote string
}

const (
	_availabilityEndpoint = "/avail"
	_loadEndpoint         = "/load"
)

func (t *Tower) getAvailability() (*FXRLayout, error) {
	as, err := t.fetchAvailability()
	if err != nil {
		return nil, err
	}

	return as.ToFXRLayout()
}

func (t *Tower) getAvailabilityForCommissioning() (*FXRLayout, error) {
	as, err := t.fetchAvailability()
	if err != nil {
		return nil, err
	}

	return as.ToFXRLayoutForCommissioning()
}

func (t *Tower) fetchAvailability() (Availability, error) {
	// only query for allowed fixtures for quicker response time
	resp, err := http.Get(t.Remote + _availabilityEndpoint + fmt.Sprintf("?%s=true", _allowedQueryKey))
	if err != nil {
		return nil, fmt.Errorf("http.Get %s: %v", t.Remote+_availabilityEndpoint, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response NOT OK: %v, %v", resp.StatusCode, resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll response body: %v", err)
	}

	var as Availability
	if err := json.Unmarshal(b, &as); err != nil {
		return nil, fmt.Errorf("json.Unmarshal response body %s: %v", string(b), err)
	}

	return as, nil
}

func (t *Tower) sendLoad(fxr *FXR, tray string, recipe *asrsapi.Recipe, tID string) error {
	stepConf, err := NewStepConfiguration(recipe.GetStepConfiguration())
	if err != nil {
		return fmt.Errorf("parse step configuation: %v", err)
	}

	fields := strings.Split(recipe.GetStep(), " - ")
	if len(fields) != 2 {
		return fmt.Errorf("recipe step '%s' is not correct format (name - version)", recipe.GetStep())
	}

	version, err := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err != nil {
		return fmt.Errorf("recipe version '%s' is not an integer", fields[1])
	}

	b, err := json.Marshal(FXRLoad{
		TransactionID: tID,
		Column:        fxr.Coord.Col,
		Level:         fxr.Coord.Lvl,
		TrayID:        tray,
		RecipeName:    strings.TrimSpace(fields[0]),
		RecipeVersion: version,
		StepType:      recipe.GetStepType(),
		Steps:         stepConf,
	})
	if err != nil {
		return fmt.Errorf("json.Marshal FXRLoad: %v", err)
	}

	resp, err := http.Post(t.Remote+_loadEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("http.Post: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("load failed, received status NOT OK: %v: %v", resp.StatusCode, resp.Status)
	}

	return nil
}
