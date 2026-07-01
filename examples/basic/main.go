package main

import (
	"context"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/log"
	"github.com/duxweb/runa/middleware/logger"
	"github.com/duxweb/runa/route"
)

func main() {
	app := runa.New()
	app.Install(
		log.Provider(log.Register(log.HTTP, log.Console(log.Pretty()))),
		route.Provider(route.Addr(":8080")),
	)
	route.Default().Use(logger.New())
	route.Default().Get("/", func(ctx *route.Context) error {
		return ctx.Text("Hello Runa")
	})
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
