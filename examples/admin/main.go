package main

import (
	"context"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/audit"
	auditmiddleware "github.com/duxweb/runa/audit/middleware"
	"github.com/duxweb/runa/console"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/security"
)

func main() {
	app := runa.New()
	app.Install(route.Provider(route.Addr(":8080")))
	admin := route.Default().Group("/admin").Name("admin")
	admin.Use(security.New(security.Disable("logger")))
	admin.Use(auditmiddleware.New(audit.Config{Writer: audit.DefaultLogWriter()}))
	admin.Get("/", func(ctx *route.Context) error {
		return ctx.Text("admin")
	}).Name("dashboard")
	console.Mount(route.Default().Group("/__runa"), app)
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
