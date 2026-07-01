package excelize

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	xlsx "github.com/xuri/excelize/v2"
)

type user struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type userQuery struct{}

type userStore struct {
	items    map[string]*user
	exported bool
	created  int
}

func newUserStore() *userStore {
	return &userStore{items: map[string]*user{
		"1": {ID: "1", Name: "old", Status: 1},
		"2": {ID: "2", Name: "new", Status: 0},
	}}
}

func (store *userStore) Query(*crud.Context[user]) (userQuery, error) { return userQuery{}, nil }
func (store *userStore) List(*crud.Context[user], userQuery) ([]*user, core.ListMeta, error) {
	return []*user{store.items["1"], store.items["2"]}, nil, nil
}
func (store *userStore) Show(ctx *crud.Context[user], query userQuery) (*user, error) {
	return nil, nil
}
func (store *userStore) Create(ctx *crud.Context[user]) (*user, error) {
	store.created++
	id := strconv.Itoa(store.created + 2)
	model := *ctx.Model
	model.ID = id
	store.items[id] = &model
	return &model, nil
}
func (store *userStore) Edit(ctx *crud.Context[user], query userQuery) (*user, error) {
	return ctx.Model, nil
}
func (store *userStore) Store(ctx *crud.Context[user], query userQuery, fields []string) (*user, error) {
	return ctx.Model, nil
}
func (store *userStore) Delete(ctx *crud.Context[user], query userQuery) error { return nil }
func (store *userStore) Tx(ctx *crud.Context[user], fn func(*crud.Context[user]) error) error {
	return fn(ctx)
}
func (store *userStore) Export(ctx *crud.Context[user], query userQuery, batch int, fn func(models []*user) error) error {
	store.exported = true
	return fn([]*user{store.items["1"], store.items["2"]})
}

func TestXLSXExport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newUserStore()
	crud.New[user, userQuery](resource.New(group, "/users").Name("user"), store).
		Actions(crud.ListAction).
		Export[exportRow](func(c *crud.Context[user], model *user) (exportRow, error) {
		return exportRow{ID: model.ID, Name: model.Name}, nil
	}, func(e *crud.Exporter[user, exportRow]) error {
		e.Name("users").Formats("xlsx", "csv")
		e.Field("id").Title("ID")
		e.Field("name").Title("名称")
		return nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/export?format=xlsx", nil))
	if response.Code != http.StatusOK || !store.exported {
		t.Fatalf("export code=%d size=%d exported=%v", response.Code, response.Body.Len(), store.exported)
	}
	if !strings.Contains(response.Header().Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("content-type = %q", response.Header().Get("Content-Type"))
	}
	file, err := xlsx.OpenReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer file.Close()
	rows, err := file.GetRows(file.GetSheetName(0))
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if rows[0][0] != "ID" || rows[1][1] != "old" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestXLSXImport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newUserStore()
	crud.New[user, userQuery](resource.New(group, "/users").Name("user"), store).
		Actions(crud.ListAction).
		Import(func(c *crud.Context[user], importer *crud.Importer[user]) error {
			importer.Column("名称").To(&importer.Model.Name)
			importer.Column("状态").To(&importer.Model.Status).Set[string](func(c *crud.Context[user], value string, row crud.ImportRow) (int, error) {
				if value == "启用" {
					return 1, nil
				}
				return 0, nil
			})
			return nil
		}, nil)

	file := xlsx.NewFile()
	sheet := file.GetSheetName(0)
	_ = file.SetCellValue(sheet, "A1", "名称")
	_ = file.SetCellValue(sheet, "B1", "状态")
	_ = file.SetCellValue(sheet, "A2", "王五")
	_ = file.SetCellValue(sheet, "B2", "启用")
	var body bytes.Buffer
	if err := file.Write(&body); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/users/import?format=xlsx", &body)
	request.Header.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import code=%d body=%q", response.Code, response.Body.String())
	}
	var result crud.ImportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Total != 1 || result.Success != 1 || store.items["3"].Name != "王五" || store.items["3"].Status != 1 {
		t.Fatalf("result=%#v item=%#v", result, store.items["3"])
	}
}

func TestXLSXImportFromContentType(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newUserStore()
	crud.New[user, userQuery](resource.New(group, "/users").Name("user"), store).
		Actions(crud.ListAction).
		Import(func(c *crud.Context[user], importer *crud.Importer[user]) error {
			importer.Column("名称").To(&importer.Model.Name)
			return nil
		}, nil)

	file := xlsx.NewFile()
	sheet := file.GetSheetName(0)
	_ = file.SetCellValue(sheet, "A1", "名称")
	_ = file.SetCellValue(sheet, "A2", "赵六")
	var body bytes.Buffer
	if err := file.Write(&body); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/users/import", &body)
	request.Header.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import code=%d body=%q", response.Code, response.Body.String())
	}
	if store.items["3"].Name != "赵六" {
		t.Fatalf("item=%#v", store.items["3"])
	}
}

type exportRow struct {
	ID   string `json:"id" label:"ID"`
	Name string `json:"name" label:"名称"`
}
