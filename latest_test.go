package towercontroller

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	tower "stash.teslamotors.com/rr/towerproto"
)

func newTestFixtureLocation(id string) string {
	return "CM2-63010-" + id
}

func newTestFixtureStateForFixture(fixture string) *FixtureState {
	fsm := &fixtureMessage{
		message: &tower.FixtureToTower{
			Info: &tower.Info{
				TrayBarcode:     "TESTBARCODEA",
				FixtureLocation: newTestFixtureLocation(fixture),
				RecipeName:      "TESTPROCESSSTEP",
				TransactionId:   "1",
			},
		},
		lastSeen:   time.Now(),
		dataExpiry: time.Second * 10,
	}
	fs := NewFixtureState()
	fs.operational = fsm
	fs.diagnostic = fsm
	fs.alert = fsm

	return fs
}

func newTestFixtureInfoForFixture(fixture string) *FixtureInfo {
	return &FixtureInfo{
		Name:         fixture,
		FixtureState: newTestFixtureStateForFixture(fixture),
	}
}

func newTestRegistry() map[string]*FixtureInfo {
	return map[string]*FixtureInfo{
		"01-01": newTestFixtureInfoForFixture("01-01"),
		"01-02": newTestFixtureInfoForFixture("01-02"),
		"02-01": newTestFixtureInfoForFixture("02-01"),
		"02-02": newTestFixtureInfoForFixture("02-02"),
	}
}

func TestHandleLatest(t *testing.T) {
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	testRegistry := newTestRegistry()
	router := mux.NewRouter()
	router.HandleFunc(LatestOpEndpoint, HandleLatestOp(zap.NewExample().Sugar(), testRegistry))
	router.HandleFunc(LatestDiagEndpoint, HandleLatestDiag(zap.NewExample().Sugar(), testRegistry))
	router.HandleFunc(LatestAlertEndpoint, HandleLatestAlert(zap.NewExample().Sugar(), testRegistry))

	srv := http.Server{
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

	for _, ep := range []string{"op", "diag", "alert"} {
		urlF := "http://localhost:" + fmt.Sprint(port) + "/%s/" + ep

		for _, tc := range testRegistry {
			url := fmt.Sprintf(urlF, tc.Name)
			t.Run(url, func(t *testing.T) {
				assert.True(t, strings.Contains(mustGetLatest(t, url), newTestFixtureLocation(tc.Name)))
			})
		}
	}
}

func mustGetLatest(t *testing.T, url string) string {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	jb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	return string(jb)
}
