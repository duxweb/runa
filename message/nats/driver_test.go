package nats

import (
	"testing"

	"github.com/duxweb/runa/message"
)

func TestDriverImplementsDriver(t *testing.T) {
	if _, ok := Driver(nil).(message.Driver); !ok {
		t.Fatal("driver does not implement message.Driver")
	}
}

func TestSubjectNormalizesSeparators(t *testing.T) {
	driver := Driver(nil, Prefix("test.")).(*driver)
	if got := driver.subject("device/1:status"); got != "test.device.1.status" {
		t.Fatalf("subject = %q", got)
	}
	if got := driver.subject("device/+/status"); got != "test.device.*.status" {
		t.Fatalf("subject wildcard = %q", got)
	}
	if got := driver.subject("device/#"); got != "test.device.>" {
		t.Fatalf("subject multi wildcard = %q", got)
	}
}
