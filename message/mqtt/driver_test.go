package mqtt

import (
	"testing"

	"github.com/duxweb/runa/message"
)

func TestDriverImplementsBrokerDriver(t *testing.T) {
	if _, ok := Driver(nil).(message.BrokerDriver); !ok {
		t.Fatal("driver does not implement message.BrokerDriver")
	}
}

func TestTopicNormalizesSeparatorsAndWildcards(t *testing.T) {
	driver := Driver(nil, Prefix("test/")).(*driver)
	if got := driver.topic("device.*.status"); got != "test/device/+/status" {
		t.Fatalf("topic = %q", got)
	}
	if got := driver.topic("device.**"); got != "test/device/#" {
		t.Fatalf("multi wildcard topic = %q", got)
	}
}
