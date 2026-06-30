package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/duxweb/runa/core"
)

type cookieDriver struct{}

// CookieDriver creates an encrypted cookie-only session driver.
func CookieDriver() Driver { return cookieDriver{} }

func (cookieDriver) Name() string { return DriverCookie }
func (cookieDriver) Load(context.Context, string) (core.Map, bool, error) {
	return nil, false, nil
}
func (cookieDriver) Save(context.Context, string, core.Map, time.Duration) error { return nil }
func (cookieDriver) Delete(context.Context, string) error                        { return nil }
func (cookieDriver) Close(context.Context) error                                 { return nil }

func (cookieDriver) LoadValue(_ context.Context, value string, options CookieOptions) (core.Map, bool, error) {
	plain, ok := DecodeSigned(value, options)
	if !ok {
		return nil, false, nil
	}
	plain, ok, err := DecodeEncrypted(plain, options)
	if err != nil || !ok {
		return nil, ok, err
	}
	var data core.Map
	if err := json.Unmarshal([]byte(plain), &data); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (cookieDriver) SaveValue(_ context.Context, data core.Map, _ time.Duration, options CookieOptions) (string, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	encrypted, err := EncodeEncrypted(string(body), options)
	if err != nil {
		return "", err
	}
	return EncodeSigned(encrypted, options), nil
}
