package session

import (
	"context"
	"fmt"
	"time"

	"github.com/duxweb/runa/core"
)

const flashKey = "_flash"

type Session struct {
	name      string
	id        string
	driver    Driver
	options   Options
	setCookie CookieSetter
	data      core.Map
	newID     bool
	dirty     bool
	destroy   bool
}

func newSession(name string, id string, driver Driver, options Options, data core.Map, isNew bool, setter CookieSetter) *Session {
	if data == nil {
		data = make(core.Map)
	}
	return &Session{name: name, id: id, driver: driver, options: options, data: core.CloneMap(data), newID: isNew, setCookie: setter}
}

func (session *Session) ID() string { return session.id }

func (session *Session) Get[T any](key string) (T, bool, error) {
	value, ok := session.data[key]
	if !ok {
		var zero T
		return zero, false, nil
	}
	casted, ok := core.CastOK[T](value)
	if ok {
		return casted, true, nil
	}
	var zero T
	return zero, false, fmt.Errorf("session key %s cannot cast %T", key, zero)
}

func (session *Session) Set(key string, value any) error {
	if key == "" {
		return fmt.Errorf("session key is required")
	}
	session.data[key] = value
	session.dirty = true
	return nil
}

func (session *Session) Delete(keys ...string) error {
	for _, key := range keys {
		delete(session.data, key)
	}
	if len(keys) > 0 {
		session.dirty = true
	}
	return nil
}

func (session *Session) Has(key string) bool {
	_, ok := session.data[key]
	return ok
}

func (session *Session) All() core.Map { return core.CloneMap(session.data) }

func (session *Session) Flash(key string, value any) error {
	flashes := session.flashMap()
	items, _ := flashes[key].([]any)
	items = append(items, value)
	flashes[key] = items
	session.data[flashKey] = flashes
	session.dirty = true
	return nil
}

func (session *Session) Flashes[T any](key string) ([]T, error) {
	flashes := session.flashMap()
	raw, ok := flashes[key]
	if !ok {
		return nil, nil
	}
	delete(flashes, key)
	if len(flashes) == 0 {
		delete(session.data, flashKey)
	} else {
		session.data[flashKey] = flashes
	}
	session.dirty = true
	items, ok := raw.([]any)
	if !ok {
		return nil, nil
	}
	values := make([]T, 0, len(items))
	for _, item := range items {
		value, ok := core.CastOK[T](item)
		if !ok {
			var zero T
			return nil, fmt.Errorf("session flash %s cannot cast %T", key, zero)
		}
		values = append(values, value)
	}
	return values, nil
}

func (session *Session) Regenerate(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	old := session.id
	id, err := newID()
	if err != nil {
		return err
	}
	session.id = id
	session.newID = true
	session.dirty = true
	if old != "" {
		return session.driver.Delete(ctx, old)
	}
	return nil
}

func (session *Session) Save(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	if session.destroy {
		return nil
	}
	if !session.dirty {
		return nil
	}
	if stateless, ok := session.driver.(Stateless); ok {
		value, err := stateless.SaveValue(ctx, session.data, session.options.TTL, session.options.Cookie)
		if err != nil {
			return err
		}
		session.writeCookie(value, session.options.TTL)
		session.dirty = false
		session.newID = false
		return nil
	}
	if err := session.driver.Save(ctx, session.id, core.CloneMap(session.data), session.options.TTL); err != nil {
		return err
	}
	session.writeCookie(EncodeSigned(session.id, session.options.Cookie), session.options.TTL)
	session.dirty = false
	session.newID = false
	return nil
}

func (session *Session) Destroy(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	session.destroy = true
	session.dirty = false
	session.data = make(core.Map)
	if session.setCookie != nil {
		options := session.options.Cookie
		options.MaxAge = -1
		options.Expires = time.Unix(0, 0)
		session.setCookie(session.options.CookieName, "", options)
	}
	return session.driver.Delete(ctx, session.id)
}

func (session *Session) writeCookie(value string, ttl time.Duration) {
	if session.setCookie == nil {
		return
	}
	options := session.options.Cookie
	if ttl > 0 && options.MaxAge == 0 {
		options.MaxAge = int(ttl.Seconds())
	}
	session.setCookie(session.options.CookieName, value, options)
}

func (session *Session) flashMap() core.Map {
	flashes, ok := session.data[flashKey].(core.Map)
	if ok {
		return flashes
	}
	if raw, ok := session.data[flashKey].(map[string]any); ok {
		flashes = core.Map(raw)
		return flashes
	}
	return make(core.Map)
}
