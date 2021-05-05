package protostream

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// CanaryEndpoint handles incoming requests for TC health
const CanaryEndpoint = "/canary"

// canaryResponse contains message counter metrics for the last 1, 5 and 15 minutes
type canaryResponse struct {
	Activity1m  MetricPeriod `json:"activity_1m"`
	Activity5m  MetricPeriod `json:"activity_5m"`
	Activity15m MetricPeriod `json:"activity_15m"`
}

// HandleCanary handles incoming requests to the canary endpoint
func HandleCanary(mux *http.ServeMux, logger *zap.SugaredLogger, mh *MetricsHandler) {
	mux.HandleFunc(CanaryEndpoint, func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		if r.Method != http.MethodGet {
			http.Error(w, "only GET supported for this endpoint", http.StatusBadRequest)
			return
		}

		cl := logger.With("endpoint", CanaryEndpoint, "remote", r.RemoteAddr)
		cl.Info("got request to endpoint")

		var sum5 MetricPeriod

		for _, num := range mh.metricPeriods[:5] {
			sum5.TowerToFixture += num.TowerToFixture
			sum5.FixtureToTower += num.FixtureToTower
			sum5.TauxToTower += num.TauxToTower
		}

		sum15 := MetricPeriod{
			TowerToFixture: sum5.TowerToFixture,
			FixtureToTower: sum5.FixtureToTower,
			TauxToTower:    sum5.TauxToTower,
		}

		for _, num := range mh.metricPeriods[5:] {
			sum15.TowerToFixture += num.TowerToFixture
			sum15.FixtureToTower += num.FixtureToTower
			sum15.TauxToTower += num.TauxToTower
		}

		cr := canaryResponse{
			Activity1m:  *mh.metricPeriods[0],
			Activity5m:  sum5,
			Activity15m: sum15,
		}

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
