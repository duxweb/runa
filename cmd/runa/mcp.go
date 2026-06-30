package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func mcpFlags() []cli.Flag { return nil }

func mcp(_ context.Context, _ *cli.Command) error { return runMCP() }

func runMCP() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		body, err := readMCPMessage(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			_ = writeMCP(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			_ = writeMCP(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		res := handleMCP(req)
		if req.ID == nil && res.Error == nil {
			continue
		}
		if err := writeMCP(res); err != nil {
			return err
		}
	}
}

func readMCPMessage(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	line = bytes.TrimSpace(line)
	if bytes.HasPrefix(line, []byte("Content-Length:")) {
		lengthText := strings.TrimSpace(strings.TrimPrefix(string(line), "Content-Length:"))
		length, err := strconv.Atoi(lengthText)
		if err != nil {
			return nil, err
		}
		for {
			header, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}
			if len(bytes.TrimSpace(header)) == 0 {
				break
			}
		}
		body := make([]byte, length)
		_, err = io.ReadFull(reader, body)
		return body, err
	}
	return line, nil
}

func writeMCP(response rpcResponse) error {
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func handleMCP(req rpcRequest) rpcResponse {
	res := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "runa", "version": "dev"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		res.Result = map[string]any{"tools": mcpTools()}
	case "tools/call":
		result, err := callMCPTool(req.Params)
		if err != nil {
			res.Error = &rpcError{Code: -32000, Message: err.Error()}
			return res
		}
		res.Result = result
	default:
		res.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return res
}

func mcpTools() []mcpTool {
	stringArg := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []string{"name"}}
	driverArg := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"capability": map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
		},
		"required": []string{"capability", "name"},
	}
	specArg := map[string]any{"type": "object", "properties": map[string]any{"file": map[string]any{"type": "string"}}, "required": []string{"file"}}
	emptyArg := map[string]any{"type": "object", "properties": map[string]any{}}
	return []mcpTool{
		{Name: "scaffold_module", Description: "Run runa gen module <name>", InputSchema: stringArg},
		{Name: "scaffold_resource", Description: "Run runa gen resource <name>", InputSchema: stringArg},
		{Name: "scaffold_crud", Description: "Run runa gen crud <name>", InputSchema: stringArg},
		{Name: "scaffold_provider", Description: "Run runa gen provider <name>", InputSchema: stringArg},
		{Name: "scaffold_capability", Description: "Run runa gen capability <name>", InputSchema: stringArg},
		{Name: "scaffold_driver", Description: "Run runa gen driver <capability> <name>", InputSchema: driverArg},
		{Name: "scaffold_spec", Description: "Run runa gen spec <file>", InputSchema: specArg},
		{Name: "list_routes", Description: "Run application route:list --json", InputSchema: emptyArg},
		{Name: "list_queues", Description: "Run application queue:list --json", InputSchema: emptyArg},
		{Name: "inspect_app", Description: "Run application inspect command", InputSchema: emptyArg},
		{Name: "validate_config", Description: "Run application config:show --json", InputSchema: emptyArg},
		{Name: "check_project", Description: "Run runa doctor --json", InputSchema: emptyArg},
		{Name: "query_docs", Description: "Return generated llms.txt content", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}},
	}
}

func callMCPTool(raw json.RawMessage) (any, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	text, err := runMCPTool(params.Name, params.Arguments)
	if err != nil {
		return nil, err
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}, nil
}

func runMCPTool(name string, args map[string]any) (string, error) {
	switch name {
	case "scaffold_module":
		return runSelf("gen", "module", stringArg(args, "name"))
	case "scaffold_resource":
		return runSelf("gen", "resource", stringArg(args, "name"))
	case "scaffold_crud":
		return runSelf("gen", "crud", stringArg(args, "name"))
	case "scaffold_provider":
		return runSelf("gen", "provider", stringArg(args, "name"))
	case "scaffold_capability":
		return runSelf("gen", "capability", stringArg(args, "name"))
	case "scaffold_driver":
		return runSelf("gen", "driver", stringArg(args, "capability"), stringArg(args, "name"))
	case "scaffold_spec":
		return runSelf("gen", "spec", stringArg(args, "file"))
	case "list_routes":
		return runGoApp("route:list", "--json")
	case "list_queues":
		return runGoApp("queue:list", "--json")
	case "inspect_app":
		return runGoApp("inspect")
	case "validate_config":
		return runGoApp("config:show", "--json")
	case "check_project":
		issues := runDoctor(".")
		body, _ := json.MarshalIndent(map[string]any{"ok": len(issues) == 0, "issues": issues}, "", "  ")
		return string(body), nil
	case "query_docs":
		path := stringArg(args, "path")
		if path == "" {
			path = "llms.txt"
		}
		body, err := os.ReadFile(path)
		return string(body), err
	default:
		return "", fmt.Errorf("unknown tool %s", name)
	}
}

func runSelf(args ...string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(self, args...)
	body, err := cmd.CombinedOutput()
	return string(body), err
}

func runGoApp(args ...string) (string, error) {
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	body, err := cmd.CombinedOutput()
	return string(body), err
}

func stringArg(args map[string]any, name string) string {
	if args == nil {
		return ""
	}
	value, _ := args[name].(string)
	return value
}
