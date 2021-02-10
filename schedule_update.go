package towercontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

var _updateScheduled bool

const (
	// UpdateEndpoint is the endpoint that handles requests for the towercontroller to restart
	// to receive a new update
	UpdateEndpoint = "/update"
	// UpdateCancelEndpoint is the endpoint that handles requests to cancel an update
	UpdateCancelEndpoint = UpdateEndpoint + "/cancel"
)

const (
	_exitDueToUpdateRequest = 2
	_forceQueryKey          = "force"
)

type updateResponse struct {
	WaitingToUpdate bool `json:"waiting_to_update"`
}

// HandleUpdateCancel cancels a current update
func HandleUpdateCancel(logger *zap.SugaredLogger, cancel chan<- struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		cl := logger.With("endpoint", UpdateCancelEndpoint, "remote", r.RemoteAddr)
		cl.Info("got request to endpoint")

		if !_updateScheduled {
			http.Error(w, "no update scheduled", http.StatusConflict)
			cl.Warn("update cancellation requested when no update was scheduled")

			return
		}

		_updateScheduled = false

		cancel <- struct{}{}
	}
}

// HandleUpdate is the handler for hte endpoint that schedules a restart/update on the tower controller
func HandleUpdate(logger *zap.SugaredLogger, cancel chan struct{}, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		cl := logger.With("endpoint", UpdateEndpoint, "remote", r.RemoteAddr)
		cl.Info("got request to endpoint")

		if r.Method == http.MethodGet {
			jb, err := json.Marshal(updateResponse{WaitingToUpdate: _updateScheduled})
			if err != nil {
				http.Error(w, fmt.Errorf("marshal response: %v", err).Error(), http.StatusInternalServerError)
				cl.Errorw("failed to marshal response", "error", err)

				return
			}

			w.Header().Add("Content-Type", "application/json")

			if _, err = w.Write(jb); err != nil {
				http.Error(w, fmt.Errorf("write response: %v", err).Error(), http.StatusInternalServerError)
				cl.Errorw("failed to write response", "error", err)
			}

			return
		}

		if r.Method != http.MethodPost {
			cl.Errorw("method not allowed", "method", r.Method)
			http.Error(w, fmt.Errorf("method %s not allowed", r.Method).Error(), http.StatusMethodNotAllowed)

			return
		}

		if _updateScheduled {
			http.Error(w, "update already scheduled", http.StatusConflict)
			cl.Warn("update requested when one was already scheduled")

			return
		}

		values := r.URL.Query()

		force := values.Get(_forceQueryKey)
		if force == "true" {
			cl.Warn("force update requested, exiting now")

			go func() { // goroutine to allow normal http response
				time.Sleep(time.Millisecond * 100)
				os.Exit(_exitDueToUpdateRequest)
			}()
		}

		_updateScheduled = true

		go exitAfterAllAreIdle(cancel, cl, registry)
	}
}

func exitAfterAllAreIdle(cancel <-chan struct{}, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	t := time.NewTicker(time.Second * 5)

	exitIfReady(logger, registry)

	for {
		select {
		case <-cancel:
			logger.Info("update canceled, monitor routine exiting")
			return
		case <-t.C:
			exitIfReady(logger, registry)
		}
	}
}

func exitIfReady(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	need := len(registry)

	var got int

	for _, fi := range registry {
		status := fi.Avail.Status()
		if status == StatusUnknown || status == StatusWaitingForReservation || status == StatusUnloading {
			logger.Debugw("fixture is idle", "fixture", fi.Name)
			got++
		} else {
			logger.Debugw("fixture is not idle", "fixture", fi.Name, "state", fi.Avail.Status())
		}

		if got == need {
			logger.Info("all fixtures idle, exiting for update")
			os.Exit(_exitDueToUpdateRequest)
		}
	}

	logger.Infow("waiting on fixtures to go back to idle", "total", need, "num_idle", got, "num_waiting_on", need-got)
}
