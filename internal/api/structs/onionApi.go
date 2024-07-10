package structs

import (
	"encoding/base64"
	"encoding/json"
	pl "github.com/HannahMarsh/PrettyLogger"
)

type Onion struct {
	To        string
	From      string
	Layer     int
	IsMessage bool
	Onion     string
}

func NewOnion(msg Message, layer int) (Onion, error) {
	mBytes, err := json.Marshal(msg)
	if err != nil {
		return Onion{}, pl.WrapError(err, "error marshalling message")
	}
	return Onion{
		To:        msg.To,
		From:      "",
		Layer:     layer,
		IsMessage: true,
		Onion:     base64.StdEncoding.EncodeToString(mBytes),
	}, nil
}

func (o Onion) AddLayer(receiver string) (Onion, error) {
	o.From = receiver
	oBytes, err := json.Marshal(o)
	if err != nil {
		return Onion{}, pl.WrapError(err, "error marshalling onion")
	}
	return Onion{
		To:        receiver,
		From:      "",
		Layer:     o.Layer - 1,
		IsMessage: false,
		Onion:     base64.StdEncoding.EncodeToString(oBytes),
	}, nil
}

func (o Onion) Peel() (Onion, Message, error) {
	if o.IsMessage {
		mBytes, err := base64.StdEncoding.DecodeString(o.Onion)
		if err != nil {
			return Onion{}, Message{}, pl.WrapError(err, "error decoding onion")
		}
		var m Message
		if err = json.Unmarshal(mBytes, &m); err != nil {
			return Onion{}, Message{}, pl.WrapError(err, "error unmarshalling onion")
		}
		return Onion{}, m, nil
	}
	var peeled Onion
	oBytes, err := base64.StdEncoding.DecodeString(o.Onion)
	if err != nil {
		return Onion{}, Message{}, pl.WrapError(err, "error decoding onion")
	}
	if err = json.Unmarshal(oBytes, &peeled); err != nil {
		return Onion{}, Message{}, pl.WrapError(err, "error unmarshalling onion")
	}
	return peeled, Message{}, nil
}
