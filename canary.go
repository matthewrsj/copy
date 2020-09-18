package towercontroller

import (
	"encoding/json"
	"net/http"
	"sort"

	"go.uber.org/zap"
)

const _canaryEndpoint = "/canary"

type canaryResponse struct {
	FixturesUp   []string `json:"fixtures_broadcasting"`
	FixturesDown []string `json:"fixtures_not_broadcasting"`
}

// HandleCanary handles incoming requests to the canary endpoint
func HandleCanary(mux *http.ServeMux, registry map[string]*FixtureInfo, logger *zap.SugaredLogger) {
	mux.HandleFunc(_canaryEndpoint, func(w http.ResponseWriter, r *http.Request) {
		cl := logger.With("endpoint", _canaryEndpoint)
		cl.Infow("got request to endpont")

		if r.Method != http.MethodGet {
			cl.Error("invalid request method", "method", r.Method)
			http.Error(w, "invalid request method", http.StatusBadRequest)

			return
		}

		cr := canaryResponse{
			FixturesUp:   []string{},
			FixturesDown: []string{},
		}

		for name, info := range registry {
			if _, err := info.FixtureState.GetOp(); err != nil {
				cr.FixturesDown = append(cr.FixturesDown, name)
			} else {
				cr.FixturesUp = append(cr.FixturesUp, name)
			}
		}

		sort.Slice(cr.FixturesUp, func(i, j int) bool {
			return cr.FixturesUp[i] < cr.FixturesUp[j]
		})

		sort.Slice(cr.FixturesDown, func(i, j int) bool {
			return cr.FixturesDown[i] < cr.FixturesDown[j]
		})

		cl = cl.With("fixture_status", cr)

		w.Header().Set("Content-Type", "application/json")
		jb, err := json.Marshal(cr)
		if err != nil {
			cl.Errorw("unable to marshal canary response", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if _, err := w.Write(jb); err != nil {
			cl.Errorw("unable to write canary response", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		cl.Info("responded to request to endpoint")
	})
}
