package database

import (
	"context"
	"errors"
	"testing"

	"github.com/duxweb/runa/core"
)

type fakeDriver struct {
	db *fakeDB
}

func (driver fakeDriver) Open(context.Context, Config) (Database, error) {
	return driver.db, nil
}

type fakeDB struct {
	name   string
	closed bool
	pinged bool
	err    error
}

func (db *fakeDB) Name() string { return db.name }
func (db *fakeDB) Kind() string { return "fake" }
func (db *fakeDB) Raw() any     { return db }
func (db *fakeDB) Ping(context.Context) error {
	db.pinged = true
	return nil
}
func (db *fakeDB) Close(context.Context) error {
	db.closed = true
	return db.err
}
func (db *fakeDB) Info() Info {
	return Info{Name: db.name, Kind: "fake", Dialect: "memory", Meta: core.Map{"ok": true}}
}

func TestRegistryOpenGetPingClose(t *testing.T) {
	db := &fakeDB{name: "default"}
	registry := New()
	registry.RegisterDriver(DefaultName, fakeDriver{db: db})
	if err := registry.Open(context.Background(), nil); err != nil {
		t.Fatalf("open: %v", err)
	}
	if registry.Get(DefaultName) != db {
		t.Fatalf("db mismatch")
	}
	if err := registry.Ping(context.Background(), DefaultName); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !db.pinged {
		t.Fatal("db not pinged")
	}
	if info := registry.Info()[0]; info.Status != "open" || info.Kind != "fake" || info.Dialect != "memory" {
		t.Fatalf("info = %#v", info)
	}
	if err := registry.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !db.closed {
		t.Fatal("db not closed")
	}
}

type errorDriver struct{}

func (errorDriver) Open(context.Context, Config) (Database, error) {
	return nil, errors.New("open failed")
}

func TestRegistryOpenError(t *testing.T) {
	registry := New()
	registry.RegisterDriver(DefaultName, errorDriver{})
	if err := registry.Open(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
	if info := registry.Info()[0]; info.Status != "error" {
		t.Fatalf("info = %#v", info)
	}
}

func TestRegistryCloseJoinsErrors(t *testing.T) {
	firstErr := errors.New("first close failed")
	secondErr := errors.New("second close failed")
	registry := New()
	registry.RegisterDriver("first", fakeDriver{db: &fakeDB{name: "first", err: firstErr}})
	registry.RegisterDriver("second", fakeDriver{db: &fakeDB{name: "second", err: secondErr}})
	if err := registry.Open(context.Background(), nil); err != nil {
		t.Fatalf("open: %v", err)
	}
	err := registry.Close(context.Background())
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("close err = %v", err)
	}
}
