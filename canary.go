package towercontroller

import (
	"encoding/json"
	"net/http"
	"sort"

	"go.uber.org/zap"
)

// CanaryEndpoint handles incoming requests for TC health
const CanaryEndpoint = "/canary"

type canaryResponse struct {
	FixturesUp   []string `json:"fixtures_broadcasting"`
	FixturesDown []string `json:"fixtures_not_broadcasting"`
	VersionInfo  Versions `json:"versions"`
}

// Versions contains all the version information of towercontroller
type Versions struct {
	GitCommit  string `json:"git_commit"`
	GitBranch  string `json:"git_branch"`
	GitState   string `json:"git_state"`
	GitSummary string `json:"git_summary"`
	BuildDate  string `json:"build_date"`
	Version    string `json:"version"`
}

// HandleCanary handles incoming requests to the canary endpoint
func HandleCanary(logger *zap.SugaredLogger, registry map[string]*FixtureInfo, versions Versions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		cl := logger.With("endpoint", CanaryEndpoint, "remote", r.RemoteAddr)
		cl.Debug("got request to endpoint")

		cr := canaryResponse{
			FixturesUp:   []string{},
			FixturesDown: []string{},
			VersionInfo:  versions,
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

		cl.Debug("responded to request to endpoint")
	}
}
