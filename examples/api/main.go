package main

import (
	"context"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/openapi"
	"github.com/duxweb/runa/route"
)

type GetUserInput struct {
	ID string `param:"id"`
}

type GetUserOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := runa.New()
	app.Install(route.Provider(route.Addr(":8080")), openapi.Provider(openapi.Register("api", openapi.JSON("/openapi.json"))))
	route.Get[GetUserInput, GetUserOutput](route.Default(), "/users/{id}", func(ctx *route.Context, input *GetUserInput) (*GetUserOutput, error) {
		return &GetUserOutput{ID: input.ID, Name: "Runa"}, nil
	}).Name("user.show").Summary("用户详情").Tags("User")
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
