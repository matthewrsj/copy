package cdcontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

// AisleManager manages a round robin of aisles
type AisleManager struct {
	aisleRequestCount int
	allowedAisles     []string
	roundRobin        []string
	nextAisleName     string
	mx                *sync.Mutex
}

// NewAisleManager returns a new AisleManager pointer
func NewAisleManager() *AisleManager {
	return &AisleManager{
		allowedAisles: []string{},
		roundRobin:    []string{},
		mx:            &sync.Mutex{},
	}
}

// GetNextAisleName returns the next aisle in the round robin or the aisle set by SetNextAisle.
// This function has the side effect of clearing the aisle name set by SetNextAisle so the
// round robin will continue.
func (am *AisleManager) GetNextAisleName() string {
	am.mx.Lock()
	defer am.mx.Unlock()

	if am.nextAisleName != "" {
		next := am.nextAisleName
		am.nextAisleName = ""

		return next
	}

	if len(am.roundRobin) == 0 {
		return ""
	}

	defer func() { am.aisleRequestCount++ }()

	return am.roundRobin[am.aisleRequestCount%len(am.roundRobin)]
}

// PeekNextAisleName returns the aisle that will be next returned from GetNextAisleName
// but does not advance the round robin.
func (am *AisleManager) PeekNextAisleName() string {
	if am.nextAisleName != "" {
		return am.nextAisleName
	}

	if len(am.roundRobin) == 0 {
		return ""
	}

	return am.roundRobin[am.aisleRequestCount%len(am.roundRobin)]
}

// AisleInRoundRobin returns a boolean representing whether the aisle is in the roundRobin list
func (am *AisleManager) AisleInRoundRobin(aisle string) bool {
	return stringInSlice(aisle, am.roundRobin) >= 0
}

// AisleAllowed returns a boolean representing whether the aisle is in the allowedAisles list
func (am *AisleManager) AisleAllowed(aisle string) bool {
	return stringInSlice(aisle, am.allowedAisles) >= 0
}

func stringInSlice(s string, ss []string) int {
	for i := range ss {
		if ss[i] == s {
			return i
		}
	}

	return -1
}

// SetNextAisle sets the next aisle that will be returned from GetNextAisleName.
// This just interrupts the round robin, which will continue where it left off.
func (am *AisleManager) SetNextAisle(aisle string) error {
	am.mx.Lock()
	defer am.mx.Unlock()

	if !am.AisleInRoundRobin(aisle) {
		return fmt.Errorf("aisle '%s' is not a valid aisle name", aisle)
	}

	am.nextAisleName = aisle

	return nil
}

// AddAisleToRoundRobin adds the aisle to the round robin if the aisle is
// allowed and it is not currently in the round robin.
func (am *AisleManager) AddAisleToRoundRobin(aisles ...string) error {
	am.mx.Lock()
	defer am.mx.Unlock()

	uniques := make(map[string]struct{})
	for _, aisle := range aisles {
		uniques[aisle] = struct{}{}
	}

	if len(uniques) < len(aisles) {
		return errors.New("duplicates are not allowed")
	}

	for _, aisle := range aisles {
		if !am.AisleAllowed(aisle) {
			return fmt.Errorf("aisle '%s' is not a valid aisle name", aisle)
		}

		if am.AisleInRoundRobin(aisle) {
			return fmt.Errorf("aisle '%s' already in round robin", aisle)
		}
	}

	am.roundRobin = append(am.roundRobin, aisles...)

	return nil
}

// RemoveAisleFromRoundRobin removes the aisle from the round robin list.
// An error is returned if the aisle does not exist in the list.
// This function may partially pass and remove all aisles passed until an error
// occurred.
func (am *AisleManager) RemoveAisleFromRoundRobin(aisles ...string) error {
	// lock when we retrieve index so it doesn't get changed before we delete
	am.mx.Lock()
	defer am.mx.Unlock()

	for _, aisle := range aisles {
		i := stringInSlice(aisle, am.roundRobin)
		if i < 0 {
			return fmt.Errorf("aisle '%s' not in round robin", aisle)
		}

		am.roundRobin = am.roundRobin[:i+copy(am.roundRobin[i:], am.roundRobin[i+1:])]
	}

	return nil
}

// SetAllowedAisles sets the allowed aisles list
func (am *AisleManager) SetAllowedAisles(aisles []string) {
	am.mx.Lock()
	am.allowedAisles = aisles
	am.mx.Unlock()
}

// RoundRobin returns the round robin list
func (am *AisleManager) RoundRobin() []string {
	return am.roundRobin
}

// AllowedAisles returns the allowed aisles list
func (am *AisleManager) AllowedAisles() []string {
	return am.allowedAisles
}

// AisleResponse returns the AisleResponse for the AisleManager
func (am *AisleManager) AisleResponse() AisleResponse {
	return AisleResponse{
		RoundRobin: am.RoundRobin(),
		AllAisles:  am.AllowedAisles(),
		NextAisle:  am.PeekNextAisleName(),
	}
}

// MarshalAisleResponse returns the json marshaled AisleResponse representing the
// state of the AisleManager
func (am *AisleManager) MarshalAisleResponse() ([]byte, error) {
	return json.Marshal(am.AisleResponse())
}

const _aisleRequestEndpoint = "/aisle"

const (
	_arAddToRoundRobin      = "ADD"
	_arRemoveFromRoundRobin = "REMOVE"
)

// AisleRequest is the request made to the aisle request endpoint
type AisleRequest struct {
	Aisle       string `json:"aisle"`
	RequestType string `json:"request"`
}

// AisleResponse is returned to requests to the aisle request endpoint
type AisleResponse struct {
	RoundRobin []string `json:"round_robin"`
	AllAisles  []string `json:"all_aisles"`
	NextAisle  string   `json:"next_aisle"`
}

// HandleAisleRequest handles incoming requests to the aisle request endpoint.
// This endpoint returns the current state of the aisle manager.
func HandleAisleRequest(mux *http.ServeMux, logger *zap.SugaredLogger, am *AisleManager) {
	mux.HandleFunc(_aisleRequestEndpoint, func(w http.ResponseWriter, r *http.Request) {
		cl := logger.With("endpoint", _aisleRequestEndpoint)
		cl.Info("got request to endpoint")

		w.Header().Add("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			cl.Debug("received GET request")

			if err := aisleRespond(am, w); err != nil {
				logger.Error(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		case http.MethodPost:
			cl.Debug("received POST request")

			req, err := getAisleRequest(r)
			if err != nil {
				logger.Error(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			var operation func(...string) error

			switch req.RequestType {
			case _arAddToRoundRobin:
				operation = am.AddAisleToRoundRobin
			case _arRemoveFromRoundRobin:
				operation = am.RemoveAisleFromRoundRobin
			default:
				logger.Error("invalid aisle request type", "type", req.RequestType)
				http.Error(w, "invalid aisle request type", http.StatusBadRequest)

				return
			}

			if err = operation(req.Aisle); err != nil {
				logger.Error(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			if err = aisleRespond(am, w); err != nil {
				logger.Error(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		default:
			logger.Error("invalid request method", "method", r.Method)
			http.Error(w, "invalid request method", http.StatusBadRequest)

			return
		}

		cl.Info("responded to request")
	})
}

func aisleRespond(am *AisleManager, w io.Writer) error {
	jb, err := am.MarshalAisleResponse()
	if err != nil {
		return fmt.Errorf("marshal aisle response: %v", err)
	}

	if _, err = w.Write(jb); err != nil {
		return fmt.Errorf("write aisle response: %v", err)
	}

	return nil
}

func getAisleRequest(r *http.Request) (AisleRequest, error) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return AisleRequest{}, err
	}

	var req AisleRequest
	err = json.Unmarshal(buf, &req)

	return req, err
}
