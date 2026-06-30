package session

import (
	"context"
	"testing"
	"time"

	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
)

func TestMemoryDriverAndSessionFlow(t *testing.T) {
	registry := New()
	registry.Session("test", Use(DriverMemory), CookieName("sid"), TTL(time.Minute))
	var cookieName, cookieValue string
	sess, err := registry.Load(context.Background(), "test", "", func(name string, value string, _ CookieOptions) {
		cookieName = name
		cookieValue = value
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := sess.Set("user_id", "42"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := sess.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if cookieName != "sid" || cookieValue == "" {
		t.Fatalf("cookie not written: %s %s", cookieName, cookieValue)
	}
	reloaded, err := registry.Load(context.Background(), "test", cookieValue, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	userID, ok, err := reloaded.Get[string]("user_id")
	if err != nil || !ok || userID != "42" {
		t.Fatalf("unexpected user id=%q ok=%v err=%v", userID, ok, err)
	}
	old := reloaded.ID()
	if err := reloaded.Regenerate(context.Background()); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if reloaded.ID() == old || reloaded.ID() == "" {
		t.Fatal("expected regenerated id")
	}
	if err := reloaded.Destroy(context.Background()); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	missing, err := registry.Load(context.Background(), "test", cookieValue, nil)
	if err != nil {
		t.Fatalf("load after destroy: %v", err)
	}
	if missing.Has("user_id") {
		t.Fatal("destroyed session should not contain data")
	}
}

func TestFlashConsumesValues(t *testing.T) {
	driver := MemoryDriver()
	id, _ := newID()
	sess := newSession("test", id, driver, applyOptions(DefaultCookieOptions()), nil, true, nil)
	if err := sess.Flash("notice", "saved"); err != nil {
		t.Fatalf("flash: %v", err)
	}
	values, err := sess.Flashes[string]("notice")
	if err != nil {
		t.Fatalf("flashes: %v", err)
	}
	if len(values) != 1 || values[0] != "saved" {
		t.Fatalf("unexpected flashes: %#v", values)
	}
	values, err = sess.Flashes[string]("notice")
	if err != nil || len(values) != 0 {
		t.Fatalf("flash should be consumed: %#v err=%v", values, err)
	}
}

func TestCacheSessionDriver(t *testing.T) {
	pool := cache.New().MustOf[core.Map](cache.Session)
	registry := New()
	registry.RegisterDriver(DriverCache, CacheDriver(DriverCache, pool))
	registry.Session("cache", Use(DriverCache), CookieName("sid"))
	var cookieValue string
	sess, err := registry.Load(context.Background(), "cache", "", func(_ string, value string, _ CookieOptions) {
		cookieValue = value
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = sess.Set("role", "admin")
	if err := sess.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := registry.Load(context.Background(), "cache", cookieValue, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	role, ok, err := reloaded.Get[string]("role")
	if err != nil || !ok || role != "admin" {
		t.Fatalf("unexpected role=%q ok=%v err=%v", role, ok, err)
	}
}

func TestCookieSessionDriver(t *testing.T) {
	registry := New()
	registry.Cookie(SignKey([]byte("sign")), EncryptKey([]byte("encrypt")))
	registry.RegisterDriver(DriverCookie, CookieDriver())
	registry.Session("cookie", Use(DriverCookie), CookieName("sid"))
	var cookieValue string
	sess, err := registry.Load(context.Background(), "cookie", "", func(_ string, value string, _ CookieOptions) {
		cookieValue = value
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_ = sess.Set("name", "runa")
	if err := sess.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if cookieValue == "" {
		t.Fatal("expected cookie payload")
	}
	reloaded, err := registry.Load(context.Background(), "cookie", cookieValue, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	name, ok, err := reloaded.Get[string]("name")
	if err != nil || !ok || name != "runa" {
		t.Fatalf("unexpected name=%q ok=%v err=%v", name, ok, err)
	}
	tampered := cookieValue[:len(cookieValue)-1] + "x"
	invalid, err := registry.Load(context.Background(), "cookie", tampered, nil)
	if err != nil {
		t.Fatalf("tampered load should not error: %v", err)
	}
	if invalid.Has("name") {
		t.Fatal("tampered cookie should not load data")
	}
}

func TestHostCookieNameForcesSecureRootPathAndNoDomain(t *testing.T) {
	registry := New()
	registry.Cookie(Secure(false), Domain("example.com"), Path("/admin"))
	registry.Session("admin", CookieName("__Host-runa_admin_session"))

	options, ok := registry.Options("admin")
	if !ok {
		t.Fatal("session options missing")
	}
	if !options.Cookie.Secure || options.Cookie.Path != "/" || options.Cookie.Domain != "" {
		t.Fatalf("cookie options = %#v", options.Cookie)
	}
}

func TestSecureCookieNameForcesSecure(t *testing.T) {
	registry := New()
	registry.Cookie(Secure(false))
	registry.Session("secure", CookieName("__Secure-runa_session"))

	options, ok := registry.Options("secure")
	if !ok {
		t.Fatal("session options missing")
	}
	if !options.Cookie.Secure {
		t.Fatalf("cookie options = %#v", options.Cookie)
	}
}
