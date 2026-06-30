package crud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
)

type inspectStore struct {
	items     []*userModel
	page      core.PageRequest
	scroll    core.ScrollRequest
	mode      PaginationMode
	sort      []SortOrder
	filters   []filter.Value
	relations []Relation
}

func (store *inspectStore) Query(ctx *Context[userModel]) (fakeQuery, error) {
	return fakeQuery{}, nil
}

func (store *inspectStore) List(ctx *Context[userModel], query fakeQuery) ([]*userModel, core.ListMeta, error) {
	store.mode = ctx.Pagination()
	store.page = ctx.Page()
	store.scroll = ctx.Scroll()
	store.sort = ctx.Sort()
	store.filters = ctx.Filters()
	store.relations = ctx.Relations()
	switch ctx.Pagination() {
	case ScrollMode:
		return store.items, core.ScrollMeta{Cursor: store.scroll.Cursor, Limit: store.scroll.Limit, Next: "next"}, nil
	case NoPageMode:
		return store.items, nil, nil
	default:
		return store.items, core.PageMeta{Page: store.page.Page, PageSize: store.page.PageSize, Total: len(store.items)}, nil
	}
}

func (store *inspectStore) Show(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	return nil, nil
}
func (store *inspectStore) Create(ctx *Context[userModel]) (*userModel, error) { return nil, nil }
func (store *inspectStore) Edit(ctx *Context[userModel], query fakeQuery) (*userModel, error) {
	return nil, nil
}
func (store *inspectStore) Store(ctx *Context[userModel], query fakeQuery, fields []string) (*userModel, error) {
	return nil, nil
}
func (store *inspectStore) Delete(ctx *Context[userModel], query fakeQuery) error { return nil }
func (store *inspectStore) Tx(ctx *Context[userModel], fn func(ctx *Context[userModel]) error) error {
	return fn(ctx)
}

func TestCrudPageSortAndFilters(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := &inspectStore{items: []*userModel{{ID: "1", Name: "one"}}}
	New[userModel, fakeQuery](resource.New(group, "/users").Name("user"), store).
		Actions(ListAction).
		Page(20, 50).
		PageFields(PageFields{Page: "p", PageSize: "limit"}).
		Sort("id", "desc").
		SortFields(SortField{Name: "name", Field: "user_name"}, SortField{Name: "status"}).
		Filters(
			filter.Eq[int]("status").Field("state"),
			filter.Like("name"),
			filter.In[string]("role"),
			filter.Between[int]("age"),
			filter.Search("keyword", "name", "email"),
			filter.Switch[int]("visible", map[int]int{1: 2}),
		).
		Relations("role", "posts.comments").
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name}
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/users?p=2&limit=200&name_sort=asc&status_sort=desc&status=1&name=run&role=admin,user&age=10,20&keyword=go&visible=1", nil)
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	if store.page.Page != 2 || store.page.PageSize != 50 || store.page.Offset != 50 {
		t.Fatalf("page = %#v", store.page)
	}
	expectedSort := []SortOrder{
		{Name: "id", Field: "id", Direction: "desc"},
		{Name: "name", Field: "user_name", Direction: "asc"},
		{Name: "status", Field: "status", Direction: "desc"},
	}
	if !reflect.DeepEqual(store.sort, expectedSort) {
		t.Fatalf("sort = %#v", store.sort)
	}
	if len(store.filters) != 6 {
		t.Fatalf("filters = %#v", store.filters)
	}
	if !reflect.DeepEqual(store.relations, []Relation{{Name: "role"}, {Name: "posts.comments"}}) {
		t.Fatalf("relations = %#v", store.relations)
	}
	if store.filters[0].Target != "state" || store.filters[0].Value != 1 {
		t.Fatalf("eq filter = %#v", store.filters[0])
	}
	if !reflect.DeepEqual(store.filters[2].Value, []any{"admin", "user"}) {
		t.Fatalf("in filter = %#v", store.filters[2])
	}
	if !reflect.DeepEqual(store.filters[3].Value, []any{10, 20}) {
		t.Fatalf("between filter = %#v", store.filters[3])
	}
	if store.filters[5].Value != 2 {
		t.Fatalf("switch filter = %#v", store.filters[5])
	}
	var body struct {
		Items []userOutput `json:"items"`
		Meta  core.Map     `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(body.Meta["page_size"].(float64)) != 50 {
		t.Fatalf("meta = %#v", body.Meta)
	}
}

func TestCrudScrollAndNoPageResponses(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	scrollStore := &inspectStore{items: []*userModel{{ID: "1", Name: "one"}}}
	New[userModel, fakeQuery](resource.New(group, "/logs").Name("log"), scrollStore).
		Actions(ListAction).
		Scroll(30, 100).
		ScrollFields(ScrollFields{Cursor: "after", Limit: "size"}).
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name}
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/logs?after=abc&size=300", nil))
	if scrollStore.mode != ScrollMode || scrollStore.scroll.Cursor != "abc" || scrollStore.scroll.Limit != 100 {
		t.Fatalf("scroll = mode:%s req:%#v", scrollStore.mode, scrollStore.scroll)
	}
	var scrollBody struct {
		Items []userOutput `json:"items"`
		Meta  core.Map     `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &scrollBody); err != nil {
		t.Fatalf("decode scroll: %v", err)
	}
	if scrollBody.Meta["next"] != "next" {
		t.Fatalf("scroll body = %#v", scrollBody)
	}

	noPageStore := &inspectStore{items: []*userModel{{ID: "1", Name: "one"}}}
	New[userModel, fakeQuery](resource.New(group, "/options").Name("option"), noPageStore).
		Actions(ListAction).
		NoPage().
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name}
	})

	response = httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/options", nil))
	var items []userOutput
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode nopage %q: %v", response.Body.String(), err)
	}
	if noPageStore.mode != NoPageMode || len(items) != 1 {
		t.Fatalf("nopage mode=%s items=%#v", noPageStore.mode, items)
	}
}

func TestCrudScrollUsesDefaultLimitAndMax(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	scrollStore := &inspectStore{items: []*userModel{{ID: "1", Name: "one"}}}
	New[userModel, fakeQuery](resource.New(group, "/events").Name("event"), scrollStore).
		Actions(ListAction).
		Scroll(0, 0).
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name}
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/events?limit=1000", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if scrollStore.scroll.Limit != 100 {
		t.Fatalf("scroll = %#v", scrollStore.scroll)
	}
}

func TestCrudPageClampsDeepOffset(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := &inspectStore{items: []*userModel{{ID: "1", Name: "one"}}}
	New[userModel, fakeQuery](resource.New(group, "/deep").Name("deep"), store).
		Actions(ListAction).
		Page(20, 100).
		Transform[userOutput](func(c *Context[userModel], model *userModel) userOutput {
		return userOutput{ID: model.ID, Name: model.Name}
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/deep?page=999999&page_size=20", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if store.page.Offset != 10000 || store.page.Page != 501 {
		t.Fatalf("page = %#v", store.page)
	}
}

func TestCrudTreeListResponse(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	store := &inspectStore{items: []*userModel{
		{ID: "1", Name: "root"},
		{ID: "2", Name: "child", Status: 1},
		{ID: "3", Name: "other"},
	}}
	New[userModel, fakeQuery](resource.New(group, "/dept").Name("dept"), store).
		Actions(ListAction).
		Tree(TreeOptions{ID: "id", ParentID: "parent_id", Children: "children"}).
		TransformMap(func(c *Context[userModel], model *userModel) core.Map {
			parentID := ""
			if model.ID == "2" {
				parentID = "1"
			}
			return core.Map{"id": model.ID, "parent_id": parentID, "name": model.Name}
		})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dept", nil))
	var tree []core.Map
	if err := json.Unmarshal(response.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode tree %q: %v", response.Body.String(), err)
	}
	if len(tree) != 2 {
		t.Fatalf("tree = %#v", tree)
	}
	children := tree[0]["children"].([]any)
	if len(children) != 1 {
		t.Fatalf("children = %#v", children)
	}
}
