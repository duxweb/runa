package oro

import (
	orodb "github.com/duxweb/oro"
	"github.com/duxweb/runa/database"
)

// From returns the raw Oro runtime from a Runa database.
func From(db database.Database) *orodb.DB {
	if db == nil {
		return nil
	}
	value, _ := db.Raw().(*orodb.DB)
	return value
}

// Context returns the Oro runtime from a route-like context.
func Context(ctx interface {
	DB(...string) database.Database
}, name ...string) *orodb.DB {
	return From(ctx.DB(name...))
}
