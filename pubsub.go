package redis

import (
	"fmt"
	"time"
)

// PubSub implements Pub/Sub commands as described in
// http://redis.io/topics/pubsub.
type PubSub struct {
	*baseClient
}

func (c *Client) PubSub() *PubSub {
	return &PubSub{
		baseClient: &baseClient{
			opt:      c.opt,
			connPool: newSingleConnPool(c.connPool, false),
		},
	}
}

func (c *Client) Publish(channel, message string) *IntCmd {
	req := NewIntCmd("PUBLISH", channel, message)
	c.Process(req)
	return req
}

// Message received as result of a PUBLISH command issued by another client.
type Message struct {
	Channel string
	Payload string
}

func (m *Message) String() string {
	return fmt.Sprintf("Message<%s: %s>", m.Channel, m.Payload)
}

// Message matching a pattern-matching subscription received as result
// of a PUBLISH command issued by another client.
type PMessage struct {
	Channel string
	Pattern string
	Payload string
}

func (m *PMessage) String() string {
	return fmt.Sprintf("PMessage<%s: %s>", m.Channel, m.Payload)
}

// Message received after a successful subscription to channel.
type Subscription struct {
	// Can be "subscribe", "unsubscribe", "psubscribe" or "punsubscribe".
	Kind string
	// Channel name we have subscribed to.
	Channel string
	// Number of channels we are currently subscribed to.
	Count int
}

func (m *Subscription) String() string {
	return fmt.Sprintf("%s: %s", m.Kind, m.Channel)
}

func (c *PubSub) Receive() (interface{}, error) {
	return c.ReceiveTimeout(0)
}

func (c *PubSub) ReceiveTimeout(timeout time.Duration) (interface{}, error) {
	cn, err := c.conn()
	if err != nil {
		return nil, err
	}
	cn.ReadTimeout = timeout

	cmd := NewSliceCmd()
	if err := cmd.parseReply(cn.rd); err != nil {
		return nil, err
	}

	reply := cmd.Val()

	msgName := reply[0].(string)
	switch msgName {
	case "subscribe", "unsubscribe", "psubscribe", "punsubscribe":
		return &Subscription{
			Kind:    msgName,
			Channel: reply[1].(string),
			Count:   int(reply[2].(int64)),
		}, nil
	case "message":
		return &Message{
			Channel: reply[1].(string),
			Payload: reply[2].(string),
		}, nil
	case "pmessage":
		return &PMessage{
			Pattern: reply[1].(string),
			Channel: reply[2].(string),
			Payload: reply[3].(string),
		}, nil
	}

	return nil, fmt.Errorf("redis: unsupported message name: %q", msgName)
}

func (c *PubSub) subscribe(cmd string, channels ...string) error {
	cn, err := c.conn()
	if err != nil {
		return err
	}

	args := make([]interface{}, 1+len(channels))
	args[0] = cmd
	for i, channel := range channels {
		args[1+i] = channel
	}
	req := NewSliceCmd(args...)
	return cn.writeCmds(req)
}

func (c *PubSub) Subscribe(channels ...string) error {
	return c.subscribe("SUBSCRIBE", channels...)
}

func (c *PubSub) PSubscribe(patterns ...string) error {
	return c.subscribe("PSUBSCRIBE", patterns...)
}

func (c *PubSub) Unsubscribe(channels ...string) error {
	return c.subscribe("UNSUBSCRIBE", channels...)
}

func (c *PubSub) PUnsubscribe(patterns ...string) error {
	return c.subscribe("PUNSUBSCRIBE", patterns...)
}
