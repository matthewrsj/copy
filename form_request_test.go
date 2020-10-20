package towercontroller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

func TestHandleSendFormRequest(t *testing.T) {
	wsPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("localhost:%d", wsPort), Path: protostream.WSEndpoint}

	s, err := protostream.NewPublisher(u.String(), "")
	if err != nil {
		t.Fatal(err)
	}

	l, err := protostream.NewSubscriberWithSub(u.String(), "01-01")
	if err != nil {
		t.Fatal(err)
	}

	sc := make(chan *protostream.Message)

	registry := map[string]*FixtureInfo{
		"01-01": {
			Name: "01-01",
		},
	}

	router := mux.NewRouter()
	router.HandleFunc(SendFormRequestEndpoint, HandleSendFormRequest(s, zap.NewExample().Sugar(), registry))

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

	status := &tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Status: tower.FixtureStatus_FIXTURE_STATUS_FAULTED,
			},
		},
	}

	buf, err := marshalProtoEvent(status, "01-01")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		sc <- &protostream.Message{
			Msg: &mangos.Message{
				Body: buf,
			},
		}
	}()

	rxd := make(chan struct{})

	var rx *protostream.Message

	go func() {
		rx = <-l.Listen()

		close(rxd)
	}()

	rfRequest := RequestForm{
		FixtureID:   "01-01",
		FormRequest: tower.FormRequest_FORM_REQUEST_FAULT_RESET.String(),
	}

	buf, err = json.Marshal(rfRequest)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d%s", port, SendFormRequestEndpoint), "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case <-rxd:
	case <-time.After(time.Second * 4):
		t.Fatal("did not receive form requeset in 4 seconds")
	}

	var pMsg protostream.ProtoMessage
	if err = json.Unmarshal(rx.Msg.Body, &pMsg); err != nil {
		t.Fatal(err)
	}

	var fReset tower.TowerToFixture
	if err = proto.Unmarshal(pMsg.Body, &fReset); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tower.FormRequest_FORM_REQUEST_FAULT_RESET, fReset.GetRecipe().GetFormRequest())

	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/form_request", port))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)

	_ = resp.Body.Close()
}
