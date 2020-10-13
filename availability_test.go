package towercontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
	tower "stash.teslamotors.com/rr/towerproto"
)

func TestHandleAvailable(t *testing.T) {
	previousConfig := _globalConfiguration

	defer func() { _globalConfiguration = previousConfig }()

	_globalConfiguration = &Configuration{
		CellAPI: cellAPIConf{},
		Loc: location{
			Line:    "CM2",
			Process: "63",
			Aisle:   "010",
		},
		AllFixtures:     []string{"01-01", "01-02", "02-01", "02-02"},
		AllowedFixtures: []string{"01-01", "02-01"},
		CellMap:         nil,
	}

	fm := &fixtureMessage{
		message: &tower.FixtureToTower{
			Content: &tower.FixtureToTower_Op{
				Op: &tower.FixtureOperational{
					Status:          tower.FixtureStatus_FIXTURE_STATUS_IDLE,
					EquipmentStatus: tower.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION,
				},
			},
		},
		lastSeen:   time.Now(),
		dataExpiry: time.Second * 5,
	}
	fs01 := NewFixtureState()
	fs01.operational = fm

	fs02 := NewFixtureState()
	fs02.operational = fm

	registry := map[string]*FixtureInfo{
		"01-01": {
			Name:         "01-01",
			FixtureState: fs01,
			Avail: ReadyStatus{
				ready: StatusActive,
				mx:    sync.Mutex{},
			},
		},
		"02-01": {
			Name:         "02-01",
			FixtureState: fs02,
			Avail: ReadyStatus{
				ready: StatusWaitingForReservation,
				mx:    sync.Mutex{},
			},
		},
	}

	router := mux.NewRouter()
	router.HandleFunc(AvailabilityEndpoint, HandleAvailable("" /* configPath not used due to global config set */, zap.NewExample().Sugar(), registry))

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	go func() {
		if err = srv.ListenAndServe(); err != http.ErrServerClosed {
			t.Error(err)
		}
	}()

	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	time.Sleep(time.Millisecond * 200)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s?allowed=true", port, AvailabilityEndpoint))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	jb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var as cdcontroller.Availability
	if err = json.Unmarshal(jb, &as); err != nil {
		t.Fatal(err)
	}

	fxrl, err := as.ToFXRLayout()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, fxrl.GetAvail())
}
