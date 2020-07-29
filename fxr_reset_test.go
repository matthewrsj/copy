package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestHandleResetFixtureFault(t *testing.T) {
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
			SC:   sc,
		},
	}

	go HandleResetFixtureFault(s, zap.NewExample().Sugar(), registry)

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	log.Println(port)

	go func() {
		if err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			t.Error(err)
		}
	}()

	status := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Status: pb.FixtureStatus_FIXTURE_STATUS_FAULTED,
			},
		},
	}

	buf, err := proto.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}

	event := &protostream.ProtoMessage{
		Location: "01-01",
		Body:     buf,
	}

	eventBuf, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		sc <- &protostream.Message{
			Msg: &mangos.Message{
				Body: eventBuf,
			},
		}
	}()

	rxd := make(chan struct{})

	var rx *protostream.Message

	go func() {
		rx = <-l.Listen()

		close(rxd)
	}()

	rfRequest := ResetFault{FixtureID: "01-01"}

	buf, err = json.Marshal(rfRequest)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d%s", port, _resetFaultEndpoint), "application/json", bytes.NewReader(buf))
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
		t.Fatal("did not receive fault reset in 4 seconds")
	}

	var pMsg protostream.ProtoMessage
	if err := json.Unmarshal(rx.Msg.Body, &pMsg); err != nil {
		t.Fatal(err)
	}

	var fReset pb.TowerToFixture
	if err := proto.Unmarshal(pMsg.Body, &fReset); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pb.FormRequest_FORM_REQUEST_FAULT_RESET, fReset.GetRecipe().GetFormrequest())
}