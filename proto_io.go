package towercontroller

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func marshalProtoEvent(msg proto.Message, fxrName string) ([]byte, error) {
	buf, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("proto marshal message: %v", err)
	}

	sendEvent := protostream.ProtoMessage{
		Location: fxrName,
		Body:     buf,
	}

	sendBuf, err := json.Marshal(sendEvent)
	if err != nil {
		return nil, fmt.Errorf("json marshal event: %v", err)
	}

	return sendBuf, nil
}

func sendProtoMessage(publisher *protostream.Socket, msg proto.Message, fxrName string) error {
	buf, err := marshalProtoEvent(msg, fxrName)
	if err != nil {
		return fmt.Errorf("marshal proto event: %v", err)
	}

	if err := publisher.PublishTo(fxrName, buf); err != nil {
		return fmt.Errorf("publish to %s: %v", fxrName, err)
	}

	return nil
}

func unmarshalProtoMessage(lMsg *protostream.Message) (*pb.FixtureToTower, error) {
	var msg pb.FixtureToTower

	var event protostream.ProtoMessage

	if err := json.Unmarshal(lMsg.Msg.Body, &event); err != nil {
		return nil, fmt.Errorf("unmarshal json frame: %v, bytes: %s", err, string(lMsg.Msg.Body))
	}

	if err := proto.Unmarshal(event.Body, &msg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal data from FXR: %v", err)
	}

	return &msg, nil
}
