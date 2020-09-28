package towercontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

func Test_soundTheAlarm(t *testing.T) {
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	remote := fmt.Sprintf("http://localhost:%d", port)

	var rxd cdcontroller.BroadcastRequest

	router := mux.NewRouter()
	router.HandleFunc(cdcontroller.BroadcastEndpoint, func(w http.ResponseWriter, r *http.Request) {
		var body []byte

		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			_ = r.Body.Close()
		}()

		if err = json.Unmarshal(body, &rxd); err != nil {
			t.Fatal(err)
		}

		w.WriteHeader(http.StatusOK)
	})

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
		if err = srv.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()

	if err = soundTheAlarm(Configuration{
		Remote: remote,
		Loc: location{
			Line:    "CM2",
			Process: "63",
			Aisle:   "010",
		},
	}, pb.FireAlarmStatus_FIRE_ALARM_LEVEL_0,
		"CM2-63010-01-01",
		zap.NewExample().Sugar(),
	); err != nil {
		t.Fatal(err)
	}

	expected := cdcontroller.BroadcastRequest{
		Scale:     cdcontroller.ScaleAisle,
		Operation: cdcontroller.OperationPauseFormation,
		Reason:    cdcontroller.ReasonFireLevel0,
		Originator: cdcontroller.BroadcastOrigin{
			Aisle:    "010",
			Location: "CM2-63010-01-01",
		},
		ExcludeOrigin: true,
	}

	assert.EqualValues(t, expected, rxd)
}
