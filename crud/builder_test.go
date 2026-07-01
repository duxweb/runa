package crud

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/validate"
)

type userModel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type userOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type fakeQuery struct {
	ID      string
	List    bool
	Touched bool
}

type fakeStore struct {
	items   map[string]*userModel
	created int
	edited  int
	stored  []string
	deleted string
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: map[string]*userModel{
		"1": {ID: "1", Name: "old", Status: 1},
		"2": {ID: "2", Name: "two", Status: 2},
	}}
}

func (store *fakeStore) Query(ctx *Context[userModel]) (fakeQuery, error) {
	return fakeQuery{ID: ctx.Param[string]("id")}, nil
}

func (store *fakeStore) List(ctx *Context[userModel], query fakeQuery) ([]*userModel, core.ListMeta, error) {
	if !query.List {
		return nil, nil, nil
	}
	return []*userModel{store.items["1"], store.items["2"]}, core.PageMeta{Page: 1, PageSize: 20, Total: 2}, nil
}

func (store *fakeStore) Show(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	item := store.items[query.ID]
	if item == nil {
		return nil, nil
	}
	clone := *item
	return &clone, nil
}

func (store *fakeStore) Create(ctx *Context[userModel]) (*userModel, error) {
	store.created++
	ctx.Model.ID = "3"
	clone := *ctx.Model
	store.items[clone.ID] = &clone
	return ctx.Model, nil
}

func (store *fakeStore) Edit(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	store.edited++
	clone := *ctx.Model
	store.items[query.ID] = &clone
	return ctx.Model, nil
}

func (store *fakeStore) Store(ctx *Context[userModel], query fakeQuery, fields []string) (*userModel, error) {
	store.stored = append([]string(nil), fields...)
	clone := *ctx.Model
	store.items[query.ID] = &clone
	return ctx.Model, nil
}

func (store *fakeStore) Delete(ctx *Context[userModel], query fakeQuery) error {
	store.deleted = query.ID
	delete(store.items, query.ID)
	return nil
}

func (store *fakeStore) Tx(ctx *Context[userModel], fn func(ctx *Context[userModel]) error) error {
	return fn(ctx)
}

func TestCrudDefaultFlowWithFakeStore(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := newFakeStore()
	calls := []string{}
	users := resource.New(group, "/users").Name("user").Summary("用户")
	builder := New[userModel, fakeQuery](users, store).
		Actions(ListAction, ShowAction, CreateAction, EditAction, StoreAction, DeleteAction).
		Init(func(c *Context[userModel]) error {
			c.Set("tenant", "main")
			return nil
		}).
		Query(func(c *Context[userModel], query fakeQuery) (fakeQuery, error) {
			query.Touched = c.Get[string]("tenant") == "main"
			return query, nil
		}).
		ListQuery(func(c *Context[userModel], query fakeQuery) (fakeQuery, error) {
			query.List = true
			return query, nil
		}).
		Validate(func(c *Context[userModel], v *validate.Validator) {
			if c.Action == CreateAction {
				v.Field("name").Value(c.Form[string]("name")).Required("请输入名称")
			}
		}).
		Format(func(c *Context[userModel], f *Formatter[userModel]) {
			f.Field("name").To(&c.Model.Name)
			f.Field("status").To(&c.Model.Status).Actions(EditAction, StoreAction)
		}).
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name, Status: model.Status}
	}).
		Meta(func(c *Context[userModel], result Result[userModel]) core.Map {
			if c.Action == ShowAction {
				return core.Map{"detail": true}
			}
			return nil
		}).
		Before(func(c *Context[userModel]) error {
			calls = append(calls, string(c.Action)+":before")
			return nil
		}, CreateAction, EditAction, StoreAction, DeleteAction).
		After(func(c *Context[userModel]) error {
			calls = append(calls, string(c.Action)+":after")
			return nil
		}, CreateAction, EditAction, StoreAction, DeleteAction)

	if builder.Route(StoreAction) == nil {
		t.Fatal("store route missing")
	}

	response := request(registry, http.MethodGet, "/users", nil)
	var list struct {
		Items []userOutput `json:"items"`
		Meta  core.Map     `json:"meta"`
	}
	decode(t, response, &list)
	if len(list.Items) != 2 || int(list.Meta["total"].(float64)) != 2 {
		t.Fatalf("list = %#v", list)
	}

	response = request(registry, http.MethodGet, "/users/1", nil)
	var show struct {
		Data userOutput `json:"data"`
		Meta core.Map   `json:"meta"`
	}
	decode(t, response, &show)
	if show.Data.Name != "old" || show.Meta["detail"] != true {
		t.Fatalf("show = %#v", show)
	}

	response = request(registry, http.MethodPost, "/users", bytes.NewBufferString("name=new"))
	var created userOutput
	decode(t, response, &created)
	if created.ID != "3" || created.Name != "new" || store.created != 1 {
		t.Fatalf("created = %#v store=%#v", created, store)
	}

	response = request(registry, http.MethodPut, "/users/1", bytes.NewBufferString("name=edit&status=9"))
	var edited userOutput
	decode(t, response, &edited)
	if edited.Name != "edit" || edited.Status != 9 || store.edited != 1 {
		t.Fatalf("edited = %#v store=%#v", edited, store)
	}

	response = request(registry, http.MethodPatch, "/users/1", bytes.NewBufferString("status=7"))
	var stored userOutput
	decode(t, response, &stored)
	if stored.Status != 7 || !reflect.DeepEqual(store.stored, []string{"status"}) {
		t.Fatalf("stored = %#v fields=%#v", stored, store.stored)
	}

	response = request(registry, http.MethodDelete, "/users/1", nil)
	if response.Code != http.StatusOK || store.deleted != "1" {
		t.Fatalf("delete code=%d deleted=%q", response.Code, store.deleted)
	}
	expectedCalls := []string{"create:before", "create:after", "edit:before", "edit:after", "store:before", "store:after", "delete:before", "delete:after"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("calls = %#v", calls)
	}
}

func request(registry *route.Registry, method string, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body.Bytes())
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	registry.Handler().ServeHTTP(res, req)
	return res
}

func decode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
}

func TestCrudCreateValidation(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	users := resource.New(group, "/users").Name("user")
	New[userModel, fakeQuery](users, newFakeStore()).
		Actions(CreateAction).
		Validate(func(c *Context[userModel], v *validate.Validator) {
			v.Field("name").Value(c.Form[string]("name")).Required("请输入名称")
		})

	response := request(registry, http.MethodPost, "/users", bytes.NewBufferString(""))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestCrudRoutesExposeOpenAPISchema(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	builder := New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), newFakeStore()).
		Actions(ListAction, ShowAction, CreateAction, StoreAction, DeleteAction).
		Page(20, 100).
		SortFields(SortField{Name: "name"}).
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name, Status: model.Status}
	})

	list := builder.Route(ListAction)
	if list.SchemaDef == nil || len(list.SchemaDef.InputFields) < 3 {
		t.Fatalf("list schema = %#v", list.SchemaDef)
	}
	if list.SchemaDef.Response == nil || list.SchemaDef.Response.Properties["items"].Items.Properties["name"].Type != "string" {
		t.Fatalf("list response schema = %#v", list.SchemaDef.Response)
	}
	show := builder.Route(ShowAction)
	if show.SchemaDef == nil || show.SchemaDef.InputFields[0].Source != "path" || show.SchemaDef.Response.Properties["status"].Type != "integer" {
		t.Fatalf("show schema = %#v", show.SchemaDef)
	}
	create := builder.Route(CreateAction)
	if create.SchemaDef == nil || create.SchemaDef.RequestBody == nil || create.SchemaDef.Response.Properties["id"].Type != "string" {
		t.Fatalf("create schema = %#v", create.SchemaDef)
	}
	if builder.Route(DeleteAction).SchemaDef.Response.Type != "object" {
		t.Fatalf("delete schema = %#v", builder.Route(DeleteAction).SchemaDef)
	}
}

func TestCrudBatchAction(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	users := resource.New(group, "/users").Name("user")
	New[userModel, fakeQuery](users, newFakeStore()).
		Actions(ListAction).
		Batch(func(c *Context[userModel], batch BatchRequest) (any, error) {
			return core.Map{"action": batch.Action, "ids": len(batch.IDs)}, nil
		})

	req := httptest.NewRequest(http.MethodPost, "/users/batch", bytes.NewBufferString(`{"action":"delete","ids":["1","2"]}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}
