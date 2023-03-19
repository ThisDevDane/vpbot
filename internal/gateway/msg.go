package gateway

import "encoding/json"

type IncomingMsg struct {
	MsgId             string
	ChannelID         string
	UserID            string
	Content           string
	HasAttachOrEmbeds bool
	IsThread          bool
}

func (msg IncomingMsg) MarshalBinary() ([]byte, error) {
	return json.Marshal(msg)
}

type OutgoingMsg struct {
	InternalID *string
	UserDM     bool
	ChannelID  string
	UserID     string
	Content    string
	ReplyID    *string
}

func (msg OutgoingMsg) MarshalBinary() ([]byte, error) {
	return json.Marshal(msg)
}

const (
	CmdDeleteMsg = iota
)

type Command struct {
	Type      int
	ChannelID string
	MsgId     string
	Reason    string
}

func (cmd Command) MarshalBinary() ([]byte, error) {
	return json.Marshal(cmd)
}
