package lang

import "github.com/duxweb/runa/core"

// Replace replaces {key} placeholders with params.
func Replace(message string, params core.Map) string {
	return NewTranslator().T(message, params)
}
