package towercontroller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandleUnreserveFixture(t *testing.T) {
	urc := make(chan struct{})

	registry := map[string]*FixtureInfo{
		"01-01": {
			Name:      "01-01",
			Unreserve: urc,
			Avail: &ReadyStatus{
				ready: StatusWaitingForLoad,
				mx:    sync.Mutex{},
			},
		},
	}

	router := mux.NewRouter()
	router.HandleFunc(UnreserveFixtureEndpoint, HandleUnreserveFixture(zap.NewExample().Sugar(), registry))

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

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

	unResReq := RequestForm{FixtureID: "01-01"}

	buf, err := json.Marshal(unResReq)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		select {
		case <-registry["01-01"].Unreserve:
		case <-time.After(2 * time.Second):
			t.Error("un-reserve request never received")
		}
	}(&wg)

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d%s", port, UnreserveFixtureEndpoint), "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	registry["01-01"].Avail.Set(StatusActive)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		select {
		case <-registry["01-01"].Unreserve:
			t.Error("fixture not in waiting to load state")
		case <-time.After(2 * time.Second):
		}
	}(&wg)

	resp, err = http.Post(fmt.Sprintf("http://localhost:%d%s", port, UnreserveFixtureEndpoint), "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/unreserve", port))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)

	_ = resp.Body.Close()
}
