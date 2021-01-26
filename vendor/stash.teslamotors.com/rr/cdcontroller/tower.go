package cdcontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
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
	cl := http.Client{
		Timeout: time.Second * 10,
	}

	// only query for allowed fixtures for quicker response time
	resp, err := cl.Get(t.Remote + _availabilityEndpoint + fmt.Sprintf("?%s=true", _allowedQueryKey))
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

func (t *Tower) sendLoad(logger *zap.SugaredLogger, fxr *FXR, tray string, recipe *asrsapi.Recipe, tID string) error {
	fields := strings.Split(recipe.GetStep(), " - ")
	if len(fields) != 2 {
		logger.Errorw("recipe step is not correct format (name - version)", "step", recipe.GetStep())
		return fmt.Errorf("recipe step '%s' is not correct format (name - version)", recipe.GetStep())
	}

	version, err := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err != nil {
		logger.Errorw("recipe version is not an integer", "ver", fields[1])
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
	})
	if err != nil {
		logger.Errorw("unable to marshal FXRLoad", "error", err)
		return fmt.Errorf("json.Marshal FXRLoad: %v", err)
	}

	c := http.Client{
		Timeout: time.Second * 5,
	}

	resp, err := c.Post(t.Remote+_loadEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		logger.Errorw("post load request", "error", err)
		return fmt.Errorf("http.Post: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Errorw("request response NOT OK", "status_code", resp.StatusCode, "status", resp.Status)
		return fmt.Errorf("load failed, received status NOT OK: %v: %v", resp.StatusCode, resp.Status)
	}

	logger.Info("successfully sent load request")

	return nil
}
