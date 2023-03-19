package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	GatewayMessageChannelWildcard = "discord.msg.*"
	GatewayMessageChannelTmpl     = "discord.msg.%s"
	GatewayOutChannel             = "discord.out"
	GatewayCommandChannel         = "discord.cmd"
	GatewayInternalGossipChannel  = "internal.gossip"
)

type Client struct {
	rdb *redis.Client
	ctx context.Context

	activePubSub *redis.PubSub
}

type GatewayOpts struct {
	Host     string
	Password string
}

func CreateClient(ctx context.Context, opts GatewayOpts) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Host,
		Password: opts.Password,
		DB:       0,
	})

	return &Client{
		rdb: rdb,
		ctx: ctx,
	}
}
func GetMessageChannelKey(channelId string) string {
	return fmt.Sprintf(GatewayMessageChannelTmpl, channelId)
}

func (c *Client) PublishMessage(channel string, content any) error {
	log.Trace().Str("channel", channel).Msgf("publising: %s", content)
	status := c.rdb.Publish(c.ctx, channel, content)
	if status != nil && status.Err() != nil {
		return status.Err()
	}

	return nil
}

func (c *Client) ObtainMessageChannel(channel string) <-chan *redis.Message {
	log.Trace().Str("channel", channel).Msgf("subscribing")
	if strings.HasSuffix(channel, "*") {
		c.activePubSub = c.rdb.PSubscribe(c.ctx, channel)
	} else {
		c.activePubSub = c.rdb.Subscribe(c.ctx, channel)
	}

	return c.activePubSub.Channel()
}

func (c *Client) Close() {
	c.activePubSub.Close()
	c.rdb.Close()
}
