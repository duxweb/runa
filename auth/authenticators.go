package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/session"
)

// AuthMux tries multiple authenticators in order.
func AuthMux(items ...Authenticator) Authenticator {
	return AuthFunc(func(ctx any) (*Info, error) {
		var missing bool
		for _, item := range items {
			if item == nil {
				continue
			}
			info, err := item.Authenticate(ctx)
			if err == nil && info != nil {
				return info, nil
			}
			if err != nil && IsNoCredentials(err) {
				missing = true
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		if missing {
			return nil, ErrNoCredentials{}
		}
		return nil, ErrNoCredentials{}
	})
}

type sessionAuth struct {
	name string
}

// SessionAuth authenticates when a named session contains auth data.
func SessionAuth(name string) Authenticator {
	return sessionAuth{name: name}
}

func (auth sessionAuth) Authenticate(ctx any) (*Info, error) {
	value, ok := ctx.(interface {
		Session(...string) *session.Session
	})
	if !ok {
		return nil, ErrNoCredentials{}
	}
	raw := value.Session(auth.name)
	if raw == nil {
		return nil, ErrNoCredentials{}
	}
	data := raw.All()
	flag := core.Cast[string](data["auth"])
	if flag == "" {
		return nil, ErrNoCredentials{}
	}
	return &Info{Method: MethodSession, Data: data}, nil
}

type apiKeyAuth struct {
	sources []TokenSource
	verify  func(string) (core.Map, bool, error)
}

// APIKeyAuth authenticates a request token with a verifier callback.
func APIKeyAuth(verify func(string) (core.Map, bool, error), sources ...TokenSource) Authenticator {
	if len(sources) == 0 {
		sources = []TokenSource{Header("X-API-Key"), Query("api_key")}
	}
	return apiKeyAuth{sources: sources, verify: verify}
}

func (auth apiKeyAuth) Authenticate(ctx any) (*Info, error) {
	token := firstToken(ctx, auth.sources)
	if token == "" {
		return nil, ErrNoCredentials{}
	}
	if auth.verify == nil {
		return nil, fmt.Errorf("api key verifier is not configured")
	}
	data, ok, err := auth.verify(token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid api key")
	}
	if data == nil {
		data = make(core.Map)
	}
	data["token"] = token
	return &Info{Method: MethodAPIKey, Data: data}, nil
}

type jwtAuth struct {
	secret  []byte
	sources []TokenSource
}

// JWTAuth authenticates HS256 JWT tokens without forcing a large dependency.
func JWTAuth(secret []byte, sources ...TokenSource) Authenticator {
	if len(sources) == 0 {
		sources = []TokenSource{Header("Authorization")}
	}
	return jwtAuth{secret: append([]byte(nil), secret...), sources: sources}
}

// JWTSign signs HS256 JWT claims.
func JWTSign(secret []byte, claims core.Map) (string, error) {
	header, err := json.Marshal(core.Map{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(core.CloneMap(claims))
	if err != nil {
		return "", err
	}
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signed))
	return signed + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (auth jwtAuth) Authenticate(ctx any) (*Info, error) {
	token := firstToken(ctx, auth.sources)
	if token == "" {
		return nil, ErrNoCredentials{}
	}
	claims, err := parseJWT(token, auth.secret)
	if err != nil {
		return nil, err
	}
	claims["token"] = token
	return &Info{Method: MethodJWT, Data: claims}, nil
}

func parseJWT(token string, secret []byte) (core.Map, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid jwt")
	}
	headerBody, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var header core.Map
	if err := json.Unmarshal(headerBody, &header); err != nil {
		return nil, err
	}
	if alg := core.Cast[string](header["alg"]); alg != "HS256" {
		return nil, fmt.Errorf("invalid jwt alg")
	}
	signed := parts[0] + "." + parts[1]
	expected := hmac.New(sha256.New, secret)
	_, _ = expected.Write([]byte(signed))
	signature := base64.RawURLEncoding.EncodeToString(expected.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(parts[2])) {
		return nil, fmt.Errorf("invalid jwt signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims core.Map
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	now := core.Now().Unix()
	if exp, ok := numericClaim(claims["exp"]); ok && now >= exp {
		return nil, fmt.Errorf("jwt is expired")
	}
	if nbf, ok := numericClaim(claims["nbf"]); ok && now < nbf {
		return nil, fmt.Errorf("jwt is not valid yet")
	}
	return claims, nil
}

func numericClaim(value any) (int64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return core.CastOK[int64](typed)
	}
}
