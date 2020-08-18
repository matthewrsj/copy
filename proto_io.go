package towercontroller

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
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
