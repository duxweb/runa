package crud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	"github.com/xuri/excelize/v2"
)

type advancedStore struct {
	*fakeStore
	deletedItems map[string]*userModel
	restored     string
	destroyed    string
	exported     bool
}

func newAdvancedStore() *advancedStore {
	return &advancedStore{
		fakeStore:    newFakeStore(),
		deletedItems: map[string]*userModel{"9": {ID: "9", Name: "deleted", Status: 1}},
	}
}

func (store *advancedStore) ShowDeleted(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	item := store.deletedItems[query.ID]
	if item == nil {
		return nil, nil
	}
	clone := *item
	return &clone, nil
}

func (store *advancedStore) Restore(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	store.restored = query.ID
	item := store.deletedItems[query.ID]
	delete(store.deletedItems, query.ID)
	store.items[query.ID] = item
	return item, nil
}

func (store *advancedStore) Destroy(ctx *Context[userModel], query fakeQuery) error {
	store.destroyed = query.ID
	delete(store.deletedItems, query.ID)
	return nil
}

func (store *advancedStore) Export(ctx *Context[userModel], query fakeQuery, batch int, fn func(models []*userModel) error) error {
	store.exported = true
	return fn([]*userModel{store.items["1"], store.items["2"]})
}

func TestCrudSoftDeleteActions(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		SoftDelete().
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name, Status: model.Status}
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/users/9/restore", nil))
	if response.Code != http.StatusOK || store.restored != "9" {
		t.Fatalf("restore code=%d body=%q restored=%q", response.Code, response.Body.String(), store.restored)
	}
	var restored userOutput
	if err := json.Unmarshal(response.Body.Bytes(), &restored); err != nil {
		t.Fatalf("restore decode: %v", err)
	}
	if restored.ID != "9" {
		t.Fatalf("restored = %#v", restored)
	}

	store.deletedItems["8"] = &userModel{ID: "8", Name: "destroy"}
	response = httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/users/8/destroy", nil))
	if response.Code != http.StatusOK || store.destroyed != "8" {
		t.Fatalf("destroy code=%d body=%q destroyed=%q", response.Code, response.Body.String(), store.destroyed)
	}
}

type exportRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestCrudCSVExport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Export[exportRow](func(c *Context[userModel], model *userModel) (exportRow, error) {
		return exportRow{ID: model.ID, Name: model.Name}, nil
	}, func(e *Exporter[userModel, exportRow]) error {
		e.Name("users").Formats("csv").Batch(10)
		e.Field("id").Title("ID")
		e.Field("name").Title("名称").Set(func(c *Context[userModel], row exportRow) (any, error) {
			return strings.ToUpper(row.Name), nil
		})
		return nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/export", nil))
	if response.Code != http.StatusOK || !store.exported {
		t.Fatalf("export code=%d body=%q exported=%v", response.Code, response.Body.String(), store.exported)
	}
	if response.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("content-type = %q", response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), "ID,名称") || !strings.Contains(response.Body.String(), "1,OLD") {
		t.Fatalf("csv = %q", response.Body.String())
	}
}

func TestCrudCSVExportEscapesFormulaValues(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Export[exportRow](func(c *Context[userModel], model *userModel) (exportRow, error) {
		return exportRow{ID: model.ID, Name: "=cmd"}, nil
	}, func(e *Exporter[userModel, exportRow]) error {
		e.Name("users").Formats("csv")
		e.Field("name").Title("名称")
		return nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/export", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("export code=%d body=%q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "'=cmd") {
		t.Fatalf("csv = %q", response.Body.String())
	}
}

func TestCrudXLSXExport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Export[exportRow](func(c *Context[userModel], model *userModel) (exportRow, error) {
		return exportRow{ID: model.ID, Name: model.Name}, nil
	}, func(e *Exporter[userModel, exportRow]) error {
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
	file, err := excelize.OpenReader(bytes.NewReader(response.Body.Bytes()))
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

func TestCrudAsyncExportStart(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	started := false
	dispatched := false
	var dispatchedQueue string
	var dispatchedJob string
	var dispatchedRequest ExportRequest
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), newAdvancedStore()).
		Actions(ListAction).
		Export[exportRow](func(c *Context[userModel], model *userModel) (exportRow, error) {
		return exportRow{}, nil
	}, func(e *Exporter[userModel, exportRow]) error {
		e.Dispatch(func(ctx context.Context, queue string, job string, request ExportRequest) (string, error) {
			dispatched = true
			dispatchedQueue = queue
			dispatchedJob = job
			dispatchedRequest = request
			return "job-1", nil
		})
		e.Queue("exports", func(q *ExportQueue[userModel, exportRow]) error {
			q.Job("user.export").Start(func(c *Context[userModel], req *ExportRequest) (*ExportResult, error) {
				started = true
				req.Set("record_id", "r1")
				return &ExportResult{ID: req.Get[string]("record_id"), Status: "pending"}, nil
			})
			return nil
		})
		return nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/export", nil))
	if response.Code != http.StatusOK || !started || !dispatched {
		t.Fatalf("async code=%d body=%q started=%v dispatched=%v", response.Code, response.Body.String(), started, dispatched)
	}
	if dispatchedQueue != "exports" || dispatchedJob != "user.export" || dispatchedRequest.Get[string]("record_id") != "r1" {
		t.Fatalf("dispatch queue=%q job=%q request=%#v", dispatchedQueue, dispatchedJob, dispatchedRequest)
	}
	var result ExportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.ID != "r1" || result.Status != "pending" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCrudCSVImport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Import(func(c *Context[userModel], importer *Importer[userModel]) error {
			importer.Column("名称").To(&importer.Model.Name)
			importer.Column("状态").To(&importer.Model.Status).Set[string](func(c *Context[userModel], value string, row ImportRow) (int, error) {
				if value == "启用" {
					return 1, nil
				}
				return 0, nil
			})
			return nil
		}, func(config *ImportConfig[userModel]) error {
			config.Formats("csv").Batch(10)
			return nil
		})

	body := bytes.NewBufferString("名称,状态\n张三,启用\n李四,停用\n")
	request := httptest.NewRequest(http.MethodPost, "/users/import", body)
	request.Header.Set("Content-Type", "text/csv")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import code=%d body=%q", response.Code, response.Body.String())
	}
	var result ImportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Total != 2 || result.Success != 2 || store.created != 2 {
		t.Fatalf("result=%#v created=%d", result, store.created)
	}
	if store.items["3"].Name != "李四" || store.items["3"].Status != 0 {
		t.Fatalf("items = %#v", store.items["3"])
	}
}

func TestCrudCSVImportRejectsInvalidColumnType(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Import(func(c *Context[userModel], importer *Importer[userModel]) error {
			importer.Column("名称").To(&importer.Model.Name)
			importer.Column("状态").To(&importer.Model.Status)
			return nil
		}, func(config *ImportConfig[userModel]) error {
			config.Formats("csv")
			return nil
		})

	body := bytes.NewBufferString("名称,状态\n张三,invalid\n")
	request := httptest.NewRequest(http.MethodPost, "/users/import", body)
	request.Header.Set("Content-Type", "text/csv")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import code=%d body=%q", response.Code, response.Body.String())
	}
	var result ImportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Total != 1 || result.Success != 0 || result.Failed != 1 || store.created != 0 {
		t.Fatalf("result=%#v created=%d", result, store.created)
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "状态 类型转换失败") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestCrudXLSXImport(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newAdvancedStore()
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Import(func(c *Context[userModel], importer *Importer[userModel]) error {
			importer.Column("名称").To(&importer.Model.Name)
			importer.Column("状态").To(&importer.Model.Status).Set[string](func(c *Context[userModel], value string, row ImportRow) (int, error) {
				if value == "启用" {
					return 1, nil
				}
				return 0, nil
			})
			return nil
		}, nil)

	file := excelize.NewFile()
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
	var result ImportResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Total != 1 || result.Success != 1 || store.items["3"].Name != "王五" || store.items["3"].Status != 1 {
		t.Fatalf("result=%#v item=%#v", result, store.items["3"])
	}
}

var _ = core.Empty{}
