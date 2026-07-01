package amqp

import (
	"testing"

	"github.com/duxweb/runa/message"
)

func TestDriverImplementsDriver(t *testing.T) {
	if _, ok := Driver(nil).(message.Driver); !ok {
		t.Fatal("driver does not implement message.Driver")
	}
}
