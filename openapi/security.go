package openapi

import "github.com/duxweb/runa/route"

// OAuthFlows stores OAuth2 flow definitions.
type OAuthFlows map[string]any

// Bearer returns a bearer auth security scheme name.
func Bearer(name string) route.Security { return route.Security(name) }

// Basic returns a basic auth security scheme name.
func Basic(name string) route.Security { return route.Security("basic:" + name) }

// ApiKey returns an api key security scheme name.
func ApiKey(name string, in string, key string) route.Security {
	return route.Security("apiKey:" + in + ":" + key + ":" + name)
}

// OAuth2 returns an OAuth2 security scheme name.
func OAuth2(name string, flows OAuthFlows) route.Security { return route.Security(name) }
