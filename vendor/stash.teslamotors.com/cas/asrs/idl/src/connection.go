package asrs

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/segmentio/ksuid"
)

type MessageId string

const MessageIDNone MessageId = ""

func (m MessageId) String() string {
	k, err := ksuid.Parse(string(m))
	if err != nil {
		return string(m)
	}
	return fmt.Sprintf("%v:%s", k.Time().Format(time.RFC3339), k.String())
}

func GenerateMessageId() MessageId {
	return MessageId(ksuid.New().String())
}

func (c *Conversation) CompareMessageId(mid MessageId) bool {
	return 0 == strings.Compare(string(mid), string(c.MsgId))
}

func (c *Conversation) SetMessageId(mid MessageId) {
	c.MsgId = string(mid)
}

func (c *Conversation) Id() MessageId {
	if c == nil {
		return MessageIDNone
	}
	return MessageId(c.MsgId)
}

func BuildConversationHeader(line, name string, mid MessageId) *Conversation {
	if mid == MessageIDNone {
		mid = GenerateMessageId()
	}
	return &Conversation{
		Line:       line,
		Origin:     name,
		Originated: ptypes.TimestampNow(),
		MsgId:      string(mid),
	}
}
