package cdcontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
)

// HandleBroadcast handles broadcast requests coming from tower controller
// nolint:gocognit // just past threshold. TODO: simplify/break out
func HandleBroadcast(lg *zap.SugaredLogger, conf Configuration, aisles map[string]*Aisle) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		logger := lg.With("endpoint", BroadcastEndpoint, "remote", r.RemoteAddr)
		logger.Info("endpoint called")

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			// can't recover if we don't know what the request is
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var br BroadcastRequest
		if err := json.Unmarshal(b, &br); err != nil {
			// can't recover if we don't know what the request is
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		location := fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Station, br.Originator.Aisle, br.Originator.Location)
		logger = logger.With("originator", location, "reason", br.Reason.String(), "scale", br.Scale.String())

		// for level 0 fire response return immediately,
		// do not tell the rest of the aisle to stop their process.
		if br.Reason == ReasonFireLevel0 {
			w.WriteHeader(http.StatusOK)

			return
		}

		switch br.Scale {
		case ScaleGlobal:
			logger.Info("broadcasting to all aisles")

			// broadcast to each tower in each aisle one time
			var lastError error

			for n, aisle := range aisles {
				go func(n string, aisle *Aisle) {
					if err := broadcastToAisle(logger, aisle, b, br.Operation); err != nil {
						lastError = err
						logger.Errorw("broadcast to aisle", "error", err, "aisle", n)
					}
				}(n, aisle)
			}

			if lastError != nil {
				logger.Errorw("unable to broadcast to all aisles", "last_error", lastError)
				// do not return, still need to tell CND
				break // from switch statement
			}
		case ScaleAisle:
			logger.Info("broadcasting to aisle")

			// broadcast to each tower in originator's aisle one time
			aisle, ok := aisles[br.Originator.Aisle]
			if !ok {
				err := fmt.Errorf("invalid aisle %s", br.Originator.Aisle)
				logger.Error(err)
				// do not return, still need to tell CND
				break // from switch statement
			}

			go func() {
				if err := broadcastToAisle(logger, aisle, b, br.Operation); err != nil {
					// do not return, still need to tell CND
					logger.Errorw("broadcast to aisle", "error", err, "aisle", br.Originator.Aisle)
				}
			}()
		case ScaleTower:
			logger.Info("broadcasting to tower")

			err := backoff.Retry(sendToTowerCallback(lg, r.RemoteAddr, b), backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 10))
			if err != nil {
				logger.Errorw("broadcast to tower", "error", err)
				// do not return, still need to tell CND
				break // from switch statement
			}
			// broadcast to originator's tower one time
		case ScaleColumn:
			logger.Info("broadcasting to column")

			// broadcast to originator's column, one per FXR
			// TODO: implement when use-case arises (not needed currently)
			logger.Error("ScaleColumn not implemented")
			http.Error(w, "ScaleColumn not implemented", http.StatusNotImplemented)

			return
		default: // ScaleNone, negative, or too high (above global)
			logger.Errorw("invalid broadcast scale", "scale", br.Scale.String())
			http.Error(w, fmt.Sprintf("invalid broadcast scale %v", br.Scale), http.StatusBadRequest)

			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// broadcastToAisle will attempt to broadcast every second for 10 seconds. If it isn't successful for any aisle it will
// return an error.
func broadcastToAisle(lg *zap.SugaredLogger, aisle *Aisle, brb []byte, op BroadcastOperation) error {
	var (
		wg      sync.WaitGroup
		lastErr error
	)

	errC := make(chan error)

	go func() {
		for err := range errC {
			lastErr = err
		}
	}()

	wg.Add(len(aisle.Towers))

	for _, tower := range aisle.Towers {
		go func(t *Tower) {
			defer wg.Done()

			err := backoff.Retry(sendToTowerCallback(lg, t.Remote, brb), backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 10))
			if err != nil {
				errC <- err
			}
		}(tower)

		if op == OperationResumeFormation {
			// stagger resumes by 5 seconds so not all the air is being used at the same time
			// it may be necessary to stagger on each tower as well, or lengthen this value
			time.Sleep(time.Second * 5)
		}
	}

	wg.Wait()
	close(errC)

	return lastErr
}

func sendToTowerCallback(lg *zap.SugaredLogger, remote string, brb []byte) func() error {
	return func() error {
		return sendToTower(lg, remote, brb)
	}
}

func sendToTower(lg *zap.SugaredLogger, remote string, brb []byte) error {
	resp, err := http.Post(remote+BroadcastEndpoint, "application/json", bytes.NewReader(brb))
	if err != nil {
		lg.Warnw("POST broadcast request to tower", "error", err)
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		rb, _ := ioutil.ReadAll(resp.Body)
		err := fmt.Errorf("status NOT OK: %d (%s): %s", resp.StatusCode, resp.Status, string(rb))
		lg.Warnw("POST broadcast response from tower", "error", err)

		return err
	}

	return nil
}
