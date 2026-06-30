package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/runtime"
)

type addInput struct {
	A int `json:"a"`
	B int `json:"b"`
}

type addOutput struct {
	Sum int `json:"sum"`
}

func TestHTTPMethodCall(t *testing.T) {
	server := testServer()
	response := request(server.HTTP(), `{"jsonrpc":"2.0","method":"math.add","params":{"a":2,"b":3},"id":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.TrimSpace(response.Body.String()) != `{"jsonrpc":"2.0","result":{"sum":5},"id":1}` {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestHTTPBatchAndNotification(t *testing.T) {
	called := 0
	server := New()
	Method[addInput, addOutput](server, "math.add", func(ctx *Context, input *addInput) (*addOutput, error) {
		called++
		return &addOutput{Sum: input.A + input.B}, nil
	})
	body := `[
		{"jsonrpc":"2.0","method":"math.add","params":{"a":1,"b":2},"id":"a"},
		{"jsonrpc":"2.0","method":"math.add","params":{"a":3,"b":4}},
		{"jsonrpc":"2.0","method":"missing","id":"b"}
	]`
	response := request(server.HTTP(), body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var output []Response
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, response.Body.String())
	}
	if len(output) != 2 {
		t.Fatalf("output len = %d body=%s", len(output), response.Body.String())
	}
	if called != 2 {
		t.Fatalf("called = %d", called)
	}
	if string(output[0].ID) != `"a"` || output[0].Error != nil {
		t.Fatalf("first = %#v", output[0])
	}
	if string(output[1].ID) != `"b"` || output[1].Error == nil || output[1].Error.Code != CodeMethodNotFound {
		t.Fatalf("second = %#v", output[1])
	}
}

func TestHTTPNotificationReturnsNoContent(t *testing.T) {
	server := testServer()
	response := request(server.HTTP(), `{"jsonrpc":"2.0","method":"math.add","params":{"a":2,"b":3}}`)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestHTTPInvalidRequest(t *testing.T) {
	server := testServer()
	response := request(server.HTTP(), `{"jsonrpc":"2.0","params":{},"id":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":-32600`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestHTTPInvalidRequestWithoutIDReturnsError(t *testing.T) {
	server := testServer()
	response := request(server.HTTP(), `{"jsonrpc":"2.0","params":{}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"id":null`) || !strings.Contains(response.Body.String(), `"code":-32600`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestHTTPRejectsNonStructuredParams(t *testing.T) {
	server := testServer()
	response := request(server.HTTP(), `{"jsonrpc":"2.0","method":"math.add","params":"bad","id":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":-32600`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestHTTPNilResultIsExplicit(t *testing.T) {
	server := New()
	server.Register("noop", func(ctx *Context) (any, error) {
		return nil, nil
	})
	response := request(server.HTTP(), `{"jsonrpc":"2.0","method":"noop","id":1}`)
	if strings.TrimSpace(response.Body.String()) != `{"jsonrpc":"2.0","result":null,"id":1}` {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestDirectCall(t *testing.T) {
	server := testServer()
	result, err := server.Call(context.Background(), "math.add", addInput{A: 4, B: 6})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	output, ok := result.(*addOutput)
	if !ok || output.Sum != 10 {
		t.Fatalf("result = %#v", result)
	}
}

func TestProviderMountsHTTPAndWS(t *testing.T) {
	server := testServer()
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(server, Path("/rpc"), WSPath("/rpc/ws")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	httpServer := httptest.NewServer(routes.Handler())
	defer httpServer.Close()

	httpResponse, err := http.Post(httpServer.URL+"/rpc", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"math.add","params":{"a":5,"b":7},"id":1}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		t.Fatalf("http status = %d", httpResponse.StatusCode)
	}

	conn, _, err := websocket.Dial(context.Background(), wsURL(httpServer.URL, "/rpc/ws"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()
	if err := wsjson.Write(context.Background(), conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "math.add",
		"params":  map[string]any{"a": 8, "b": 9},
		"id":      "ws1",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var wsResponse Response
	if err := wsjson.Read(context.Background(), conn, &wsResponse); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(wsResponse.ID) != `"ws1"` || wsResponse.Error != nil {
		t.Fatalf("ws response = %#v", wsResponse)
	}
}

func TestProviderReadsConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "jsonrpc.toml", `path = "/rpc"
ws_path = "/rpc/ws"
`)
	server := testServer()
	app := runtime.New(runtime.BasePath(root))
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(server))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	httpServer := httptest.NewServer(routes.Handler())
	defer httpServer.Close()
	response, err := http.Post(httpServer.URL+"/rpc", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"math.add","params":{"a":1,"b":2},"id":1}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestMountUnderRouteGroup(t *testing.T) {
	server := testServer()
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	Mount(routes.Group("/api/rpc"), server, WebSocket("ws"))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	httpServer := httptest.NewServer(routes.Handler())
	defer httpServer.Close()

	httpResponse, err := http.Post(httpServer.URL+"/api/rpc", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"math.add","params":{"a":1,"b":9},"id":1}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		t.Fatalf("http status = %d", httpResponse.StatusCode)
	}

	conn, _, err := websocket.Dial(context.Background(), wsURL(httpServer.URL, "/api/rpc/ws"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()
	if err := wsjson.Write(context.Background(), conn, map[string]any{
		"jsonrpc": "2.0",
		"method":  "math.add",
		"params":  map[string]any{"a": 3, "b": 4},
		"id":      "mounted",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var response Response
	if err := wsjson.Read(context.Background(), conn, &response); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(response.ID) != `"mounted"` || response.Error != nil {
		t.Fatalf("response = %#v", response)
	}
}

func testServer() *Server {
	server := New()
	Method[addInput, addOutput](server, "math.add", func(ctx *Context, input *addInput) (*addOutput, error) {
		return &addOutput{Sum: input.A + input.B}, nil
	})
	return server
}

func request(handler route.Handler, body string) *httptest.ResponseRecorder {
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	routes.Post("/rpc", handler).Raw()
	if err := app.Freeze(context.Background()); err != nil {
		panic(err)
	}
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(body)))
	return response
}

func wsURL(base string, path string) string {
	return "ws" + strings.TrimPrefix(base, "http") + path
}
