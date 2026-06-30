package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
)

// Timezone returns the configured application timezone name.
func (app *App) Timezone() string {
	app.mu.Lock()
	value := app.timezone
	app.mu.Unlock()
	if value == "" {
		return core.Location().String()
	}
	return value
}

func (app *App) applyTimezone() error {
	timezone := strings.TrimSpace(app.timezone)
	if timezone == "" {
		if store, err := Invoke[*config.Store](app); err == nil && store != nil {
			timezone = strings.TrimSpace(store.GetString("app.timezone", ""))
			if timezone == "" {
				timezone = strings.TrimSpace(store.GetString("timezone", ""))
			}
		}
	}
	if timezone == "" {
		return nil
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	core.SetLocation(location)
	app.mu.Lock()
	app.timezone = timezone
	app.mu.Unlock()
	return nil
}
