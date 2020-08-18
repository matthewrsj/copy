package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"stash.teslamotors.com/rr/traycontrollers"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestHandleBroadcastRequest(t *testing.T) {
	wsPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("localhost:%d", wsPort), Path: protostream.WSEndpoint}

	s, err := protostream.NewPublisher(u.String(), "")
	if err != nil {
		t.Fatal(err)
	}

	l, err := protostream.NewSubscriberWithSub(u.String(), "01-02")
	if err != nil {
		t.Fatal(err)
	}

	registry := map[string]*FixtureInfo{
		"01-01": {
			Name: "01-01",
		},
		"01-02": {
			Name: "01-02",
		},
	}

	go HandleBroadcastRequest(s, zap.NewExample().Sugar(), registry)

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			t.Error(err)
		}
	}()

	rxd := make(chan struct{})

	var rx *protostream.Message

	go func() {
		rx = <-l.Listen()

		close(rxd)
	}()

	br := traycontrollers.BroadcastRequest{
		Scale:     traycontrollers.ScaleAisle,
		Operation: traycontrollers.OperationStopFormation,
		Reason:    traycontrollers.ReasonFireLevel0,
		Originator: traycontrollers.BroadcastOrigin{
			Aisle:    "010",
			Location: "01-01",
		},
		ExcludeOrigin: true,
		// target excluded for aisle scale
	}

	buf, err := json.Marshal(br)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d%s", port, traycontrollers.BroadcastEndpoint), "application/json", bytes.NewReader(buf))
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
		t.Fatal("did not receive form request in 4 seconds")
	}

	var pMsg protostream.ProtoMessage
	if err := json.Unmarshal(rx.Msg.Body, &pMsg); err != nil {
		t.Fatal(err)
	}

	var fStop pb.TowerToFixture
	if err := proto.Unmarshal(pMsg.Body, &fStop); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pb.FormRequest_FORM_REQUEST_STOP, fStop.GetRecipe().GetFormrequest())
}
