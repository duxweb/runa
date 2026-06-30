package route

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/duxweb/runa/core"
)

// RenderOutput renders a typed output value.
func (ctx *Context) RenderOutput(output any) error {
	if output == nil {
		return ctx.SendStatus(ctx.StatusCode(http.StatusNoContent))
	}
	value := reflect.ValueOf(output)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ctx.SendStatus(ctx.StatusCode(http.StatusNoContent))
		}
		value = value.Elem()
	}
	if value.Type() == reflect.TypeOf(core.Empty{}) {
		return ctx.SendStatus(ctx.StatusCode(http.StatusNoContent))
	}
	body := outputBody(value)
	return ctx.renderBody(ctx.StatusCode(http.StatusOK), body)
}

func outputBody(value reflect.Value) any {
	if value.Kind() == reflect.Struct {
		field := value.FieldByName("Body")
		if field.IsValid() && field.CanInterface() {
			return field.Interface()
		}
	}
	return value.Interface()
}

func (ctx *Context) renderBody(status int, body any) error {
	switch typed := body.(type) {
	case nil:
		return ctx.SendStatus(status)
	case core.Empty:
		return ctx.SendStatus(status)
	case core.JSONRaw:
		return ctx.Status(status).Blob(core.MIMEApplicationJSON, typed.Bytes())
	case []byte:
		return ctx.Status(status).Send(typed)
	case string:
		return ctx.Status(status).Text(typed)
	case core.Stream:
		return ctx.Status(status).SendStream(typed.Reader, typed.ContentType)
	case core.ViewBody:
		return ctx.Status(status).Render(typed.Name, typed.Data, typed.View)
	default:
		if raw, ok := body.(json.RawMessage); ok {
			return ctx.Status(status).Blob(core.MIMEApplicationJSON, []byte(raw))
		}
		return ctx.Status(status).JSON(body)
	}
}
