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

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestHandleSendEquipmentRequest(t *testing.T) {
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

	mux := http.NewServeMux()

	go HandleSendEquipmentRequest(mux, s, zap.NewExample().Sugar(), registry)

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

	status := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				EquipmentStatus: pb.EquipmentStatus_EQUIPMENT_STATUS_NEEDS_APPROVAL,
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

	rfRequest := RequestEquipment{
		FixtureID:        "01-01",
		EquipmentRequest: pb.EquipmentRequest_EQUIPMENT_REQUEST_SELF_TEST_APPROVED.String(),
	}

	buf, err = json.Marshal(rfRequest)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d%s", port, _sendEquipmentRequestEndpoint), "application/json", bytes.NewReader(buf))
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
		t.Fatal("did not receive equipment request in 4 seconds")
	}

	var pMsg protostream.ProtoMessage
	if err := json.Unmarshal(rx.Msg.Body, &pMsg); err != nil {
		t.Fatal(err)
	}

	var eRequest pb.TowerToFixture
	if err := proto.Unmarshal(pMsg.Body, &eRequest); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pb.EquipmentRequest_EQUIPMENT_REQUEST_SELF_TEST_APPROVED, eRequest.GetEquipmentRequest())
}
