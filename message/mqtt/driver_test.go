package mqtt

import (
	"testing"
	"time"

	"github.com/duxweb/runa/message"
)

func TestDriverImplementsDriver(t *testing.T) {
	if _, ok := Driver(nil).(message.Driver); !ok {
		t.Fatal("driver does not implement message.Driver")
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

func TestConfigCanOverrideSharedZeroValues(t *testing.T) {
	disconnect := uint(0)
	opts := options{qos: 2, retained: true, timeout: time.Second, disconnect: 250}
	featureQoS := byte(0)
	featureRetained := false
	applyMQTTConfig(&opts, mqttConfig{QoS: &featureQoS, Retained: &featureRetained, Disconnect: &disconnect})
	if opts.qos != 0 {
		t.Fatalf("qos = %d", opts.qos)
	}
	if opts.retained {
		t.Fatal("retained should be false")
	}
	if opts.disconnect != 0 {
		t.Fatalf("disconnect = %d", opts.disconnect)
	}
}
