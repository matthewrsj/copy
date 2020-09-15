package towercontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestHandleCanary(t *testing.T) {
	registry := map[string]*FixtureInfo{
		"01-01": {
			Name:         "01-01",
			FixtureState: NewFixtureState(),
		},
		"01-02": {
			Name:         "01-02",
			FixtureState: NewFixtureState(),
		},
	}

	fsGetOp := monkey.PatchInstanceMethod(
		reflect.TypeOf(&FixtureState{}),
		"GetOp",
		func(f *FixtureState) (*pb.FixtureToTower, error) {
			return &pb.FixtureToTower{}, nil
		},
	)
	defer fsGetOp.Unpatch()

	mux := http.NewServeMux()

	HandleCanary(mux, registry, zap.NewExample().Sugar())

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
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

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/canary", port))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var cr canaryResponse
	if err := json.Unmarshal(rb, &cr); err != nil {
		t.Fatal(err)
	}

	assert.EqualValues(
		t,
		canaryResponse{
			FixturesUp:   []string{"01-01", "01-02"},
			FixturesDown: []string{},
		},
		cr,
	)
}
