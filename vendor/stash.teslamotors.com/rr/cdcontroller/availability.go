package cdcontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AvailabilityEndpoint handles incoming GET requests for availability of tower/fixtures
const AvailabilityEndpoint = "/available"

const (
	_towerQueryKey   = "tower"
	_fixtureQueryKey = "fxr"
	_allowedQueryKey = "allowed"
)

// HandleAvailable is the handler for the endpoint reporting availability of fixtures
// HandleAvailable takes "tower" query argument in the form of "AISLEID-TOWERNUM"
// where AISLEID is a three digit number (010, 020) with a trailing zero
// and TOWERNUM is a two digit number (01, 02)
// nolint:gocognit // no reason to split this up
func HandleAvailable(configPath string, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Infow("got request to /available", "remote", r.RemoteAddr)

		config, err := LoadConfig(configPath)
		if err != nil {
			logger.Errorw("read configuration file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		aisles := config.Loc.Aisles
		availableTowers := make(map[string]string, len(aisles)*8) // over-allocate to max

		for aisleName, aisle := range aisles {
			for towerName, tower := range aisle {
				towerLocation := fmt.Sprintf("%s-%02d", aisleName, towerName)
				availableTowers[towerLocation] = tower.Address
			}
		}

		respondTowers := make(map[string]string)

		values := r.URL.Query()

		requestedFXR := r.URL.Query().Get(_fixtureQueryKey)
		if requestedFXR != "" {
			var rb []byte

			if rb, err = responseForSingleFixture(requestedFXR, availableTowers, config.Loc.Station); err != nil {
				logger.Errorw("generate response for single fixture", "fixture", requestedFXR, "error", err)
				http.Error(w, fmt.Errorf("generate response for single fixture: %v", err).Error(), http.StatusInternalServerError)

				return
			}

			w.Header().Add("Content-Type", "application/json")

			if _, err = w.Write(rb); err != nil {
				logger.Error("unable to write response JSON", "error", err)
				http.Error(w, fmt.Errorf("write response json: %v", err).Error(), http.StatusInternalServerError)
			}

			return
		}

		requestedTowers, ok := values[_towerQueryKey]
		if !ok {
			respondTowers = availableTowers
		} else {
			for _, requested := range requestedTowers {
				address, ok := availableTowers[requested]
				if !ok {
					http.Error(w, fmt.Sprintf("requested tower %s does not exist", requested), http.StatusBadRequest)
					logger.Errorw("requested tower does not exist", "requested", requested)

					return
				}

				respondTowers[requested] = address
			}
		}

		allowedQuery := values.Get(_allowedQueryKey)
		if allowedQuery != "" {
			allowedQuery = fmt.Sprintf("?%s=%s", _allowedQueryKey, allowedQuery)
		}

		var (
			sResp sync.Map
			wg    sync.WaitGroup
		)

		wg.Add(len(respondTowers))

		for name, address := range respondTowers {
			go getOneAvailability(name, address, allowedQuery, logger, &wg, &sResp)
		}

		wg.Wait()

		resp := make(map[string]Availability)

		sResp.Range(func(key, value interface{}) bool {
			name, ok := key.(string)
			if !ok {
				logger.Errorf("invalid type '%T' for name '%v'", key, key)
				return true // will never happen
			}

			avail, ok := value.(Availability)
			if !ok {
				logger.Errorf("invalid type '%T' for availability '%v'", value, value)
				return true // will never happen
			}

			resp[name] = avail
			return true
		})

		rb, err := json.Marshal(resp)
		if err != nil {
			logger.Error("unable to marshal response JSON", "error", err)
			http.Error(w, fmt.Errorf("mashal response json: %v", err).Error(), http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(rb); err != nil {
			logger.Error("unable to write response JSON", "error", err)
			http.Error(w, fmt.Errorf("write response json: %v", err).Error(), http.StatusInternalServerError)
		}
	}
}

func getOneAvailability(name, address, allowedQuery string, logger *zap.SugaredLogger, wg *sync.WaitGroup, sResp *sync.Map) {
	defer wg.Done()

	avail, err := getOneAvailabilitySync(address, allowedQuery)
	if err != nil {
		logger.Warnw("unable to get availability for tower", "error", err, "name", name, "address", address)
	}

	// regardless of the error we still store the empty object for response
	sResp.Store(name, avail)
}

func getOneAvailabilitySync(address, allowedQuery string) (Availability, error) {
	const getAvailabilityTimeout = time.Second

	var avail Availability

	c := http.Client{
		Timeout: getAvailabilityTimeout,
	}

	url := address + _availabilityEndpoint + allowedQuery

	r, err := c.Get(url)
	if err != nil {
		return avail, fmt.Errorf("GET tower availability for %s: %v", url, err)
	}

	defer func() {
		_ = r.Body.Close()
	}()

	var jb []byte

	if jb, err = ioutil.ReadAll(r.Body); err != nil {
		return avail, fmt.Errorf("read response body from %s: %v", url, err)
	}

	if err = json.Unmarshal(jb, &avail); err != nil {
		return avail, fmt.Errorf("unmarshal response body from %s: %v", url, err)
	}

	return avail, nil
}

func responseForSingleFixture(requested string, availableTowers map[string]string, station string) ([]byte, error) {
	var (
		fxrLoc FixtureBarcode
		avail  Availability
		colNum int
		jb     []byte
	)

	fxrLoc, err := NewFixtureBarcode(requested)
	if err != nil {
		return nil, fmt.Errorf("parse new fixture location: %v", err)
	}

	if colNum, err = strconv.Atoi(fxrLoc.Tower); err != nil {
		return nil, fmt.Errorf("parse column from fixture location: %v", err)
	}

	towerNum := int(math.Ceil(float64(colNum) / float64(2)))
	reqAddr := fmt.Sprintf("%s-%02d", strings.TrimPrefix(fxrLoc.Aisle, station), towerNum)

	address, ok := availableTowers[reqAddr]
	if !ok {
		return nil, fmt.Errorf("key %s not found in available towers %v", reqAddr, availableTowers)
	}

	avail, err = getOneAvailabilitySync(address, "" /* allowedQuery - get fixture regardless of allowed status */)
	if err != nil {
		return nil, fmt.Errorf("get availability for tower %s: %v", address, err)
	}

	single, ok := avail[requested]
	if !ok {
		return nil, fmt.Errorf("fixture %s not found in tower availability %v", requested, avail)
	}

	singleAvail := map[string]Availability{
		reqAddr: {requested: single},
	}

	if jb, err = json.Marshal(singleAvail); err != nil {
		return nil, fmt.Errorf("marshal response body: %v", err)
	}

	return jb, nil
}
