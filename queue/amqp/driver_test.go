package amqp

import (
	"testing"

	"github.com/duxweb/runa/queue"
)

func TestAMQPDriverContract(t *testing.T) {
	driver := Driver(nil)
	if driver.Name() != "amqp" {
		t.Fatalf("name = %q", driver.Name())
	}
	if _, ok := driver.(queue.Driver); !ok {
		t.Fatalf("driver does not implement queue.Driver")
	}
}

func TestAMQPOptionsDefaults(t *testing.T) {
	instance := Driver(nil, Prefix("runa"), Exchange("events"), Prefetch(0)).(*driver)
	if instance.options.prefix != "runa" || instance.options.exchange != "events" {
		t.Fatalf("options = %+v", instance.options)
	}
	if instance.options.prefetch != 1 {
		t.Fatalf("prefetch = %d", instance.options.prefetch)
	}

	instance = Driver(nil, Prefetch(8)).(*driver)
	if instance.options.prefetch != 8 {
		t.Fatalf("prefetch = %d", instance.options.prefetch)
	}
}
