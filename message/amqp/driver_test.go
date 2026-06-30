package amqp

import (
	"testing"

	"github.com/duxweb/runa/message"
)

func TestDriverImplementsBrokerDriver(t *testing.T) {
	if _, ok := Driver(nil).(message.BrokerDriver); !ok {
		t.Fatal("driver does not implement message.BrokerDriver")
	}
}
