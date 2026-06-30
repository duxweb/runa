package message

import (
	"context"
	"reflect"

	"github.com/duxweb/runa/core"
)

type brokerEntry struct {
	name    string
	options BrokerOptions
	code    []BrokerOption
}

type subscriptionEntry struct {
	broker      string
	topic       string
	consumer    string
	meta        core.Map
	codec       Codec
	payloadType reflect.Type
	payloadName string
	call        func(context.Context, Envelope, Codec) error
}
