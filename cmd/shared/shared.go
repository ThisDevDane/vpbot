package shared

import "encoding/json"

var (
	RedisAddr     string
	RedisPassword string
)

type DiscordMsg struct {
	MsgId             string
	InternalID        *string
	UserDM            bool
	ChannelID         string
	UserID            string
	Content           string
	HasAttachOrEmbeds bool
	IsThread          bool
}

func (msg DiscordMsg) MarshalBinary() ([]byte, error) {
	return json.Marshal(msg)
}

const (
	GatewayCmdDeleteMsg = iota
)

type GatewayCommand struct {
	Type      int
	ChannelID string
	MsgId     string
}

func (cmd GatewayCommand) MarshalBinary() ([]byte, error) {
	return json.Marshal(cmd)
}
