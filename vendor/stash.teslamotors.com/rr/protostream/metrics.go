package protostream

import (
	"sync"
	"time"
)

// MetricsHandler contains an array of metric periods, for how long in the past we want metrics (15 minutes/periods for now)
type MetricsHandler struct {
	mux           *sync.Mutex
	metricPeriods []*MetricPeriod
}

// MetricPeriod holds the 3 message count metrics for each time period we increment (minute for now)
type MetricPeriod struct {
	TowerToFixture int `json:"tower_to_fixture"`
	FixtureToTower int `json:"fixture_to_tower"`
	TauxToTower    int `json:"taux_to_tower"`
}

// CountFixtureToTower increments the current metric period FixtureToTower by one
func (mh *MetricsHandler) CountFixtureToTower() {
	mh.mux.Lock()
	defer mh.mux.Unlock()

	mh.metricPeriods[0].FixtureToTower++
}

// CountTowerToFixture increments the current metric period TowerToFixture by one
func (mh *MetricsHandler) CountTowerToFixture() {
	mh.mux.Lock()
	defer mh.mux.Unlock()

	mh.metricPeriods[0].TowerToFixture++
}

// CountTauxToTower increments the current metric period TauxToTower by one
func (mh *MetricsHandler) CountTauxToTower() {
	mh.mux.Lock()
	defer mh.mux.Unlock()

	mh.metricPeriods[0].TauxToTower++
}

// ShiftMetricPeriods shifts the first 14 periods over one index, deletes the 15th and sets the first period to 0
func (mh *MetricsHandler) ShiftMetricPeriods() {
	mh.mux.Lock()
	defer mh.mux.Unlock()

	mh.metricPeriods = append([]*MetricPeriod{{}}, mh.metricPeriods[:14]...)
}

// Run starts a timer that shifts the metric periods every minute
func (mh *MetricsHandler) Run() {
	for tick := time.NewTicker(time.Minute); ; <-tick.C {
		mh.ShiftMetricPeriods()
	}
}

// NewMetricsHandler initializes the struct and sets all the array values to defaults
func NewMetricsHandler() *MetricsHandler {
	mh := MetricsHandler{
		mux:           &sync.Mutex{},
		metricPeriods: make([]*MetricPeriod, 15),
	}

	for i := range mh.metricPeriods {
		mh.metricPeriods[i] = &MetricPeriod{}
	}

	return &mh
}
