package orostore

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/softdelete"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud"
	"github.com/duxweb/runa/crud/filter"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	_ "modernc.org/sqlite"
)

type storeUser struct {
	oro.Model
	softdelete.SoftDeleteFields
	Name   string
	Email  string
	Status int
	RoleID uint64
}

func (storeUser) Define(s *oro.SchemaBuilder) {
	s.Table("orostore_users")
	s.Field("Name").String()
	s.Field("Email").String()
	s.Field("Status").Int()
	s.Field("RoleID").UnsignedBigInt()
}

func (user storeUser) Role() oro.Relation {
	return oro.BelongsTo(user, "Role", "storeRole").
		ForeignKey("RoleID").
		ReferenceKey("ID")
}

type storeRole struct {
	oro.Model
	Name string
}

func (storeRole) Define(s *oro.SchemaBuilder) {
	s.Table("orostore_roles")
	s.Field("Name").String()
}

type storeUserOutput struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status int    `json:"status"`
	Role   string `json:"role,omitempty"`
}

func TestOroStoreCrudFlow(t *testing.T) {
	db := newTestDB(t)
	seedUsers(t, db)

	registry := route.New()
	users := resource.New(route.NewGroup(registry, ""), "/users").Name("user").Summary("用户")
	crud.New[storeUser](users, Store[storeUser](db)).
		Actions(crud.ListAction, crud.ShowAction, crud.CreateAction, crud.EditAction, crud.StoreAction, crud.DeleteAction).
		Key("id").
		Page(2, 10).
		Sort("id", "asc").
		SortFields(crud.SortField{Name: "name"}).
		Filters(
			filter.Eq[int]("status"),
			filter.Like("name"),
		).
		Relations("Role").
		Format(formatStoreUser).
		Transform[storeUserOutput](transformStoreUser)

	list := request(registry, http.MethodGet, "/users?status=1&name=ali", nil)
	var listBody struct {
		Items []storeUserOutput `json:"items"`
		Meta  core.Map          `json:"meta"`
	}
	decode(t, list, &listBody)
	if len(listBody.Items) != 1 || listBody.Items[0].Role != "admin" || int(listBody.Meta["total"].(float64)) != 1 {
		t.Fatalf("list = %#v", listBody)
	}

	show := request(registry, http.MethodGet, "/users/1", nil)
	var shown storeUserOutput
	decode(t, show, &shown)
	if shown.ID != 1 || shown.Name != "alice" || shown.Role != "admin" {
		t.Fatalf("show = %#v", shown)
	}

	create := request(registry, http.MethodPost, "/users", bytes.NewBufferString("name=cindy&email=cindy@example.com&status=1&role_id=1"))
	var created storeUserOutput
	decode(t, create, &created)
	if created.ID == 0 || created.Name != "cindy" {
		t.Fatalf("created = %#v", created)
	}

	edit := request(registry, http.MethodPut, "/users/1", bytes.NewBufferString("name=alice2&email=alice2@example.com&status=2&role_id=2"))
	var edited storeUserOutput
	decode(t, edit, &edited)
	if edited.Name != "alice2" || edited.Status != 2 || edited.Role != "guest" {
		t.Fatalf("edited = %#v", edited)
	}

	store := request(registry, http.MethodPatch, "/users/1", bytes.NewBufferString("status=3"))
	var stored storeUserOutput
	decode(t, store, &stored)
	if stored.Name != "alice2" || stored.Status != 3 {
		t.Fatalf("stored = %#v", stored)
	}

	deleted := request(registry, http.MethodDelete, "/users/2", nil)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%q", deleted.Code, deleted.Body.String())
	}
	hidden, err := db.Use[storeUser]().Where("ID", 2).First(t.Context())
	if err != nil {
		t.Fatalf("query hidden: %v", err)
	}
	if hidden != nil {
		t.Fatalf("soft-deleted user should be hidden: %#v", hidden)
	}
}

func TestOroStoreSoftDeleteActions(t *testing.T) {
	db := newTestDB(t)
	seedUsers(t, db)

	registry := route.New()
	users := resource.New(route.NewGroup(registry, ""), "/users").Name("user")
	crud.New[storeUser](users, Store[storeUser](db)).
		Actions(crud.ListAction).
		SoftDelete().
		Transform[storeUserOutput](transformStoreUser)

	if res := request(registry, http.MethodDelete, "/users/2", nil); res.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%q", res.Code, res.Body.String())
	}
	restore := request(registry, http.MethodPut, "/users/2/restore", nil)
	var restored storeUserOutput
	decode(t, restore, &restored)
	if restored.ID != 2 || restored.Name != "bob" {
		t.Fatalf("restored = %#v", restored)
	}
	if res := request(registry, http.MethodDelete, "/users/2", nil); res.Code != http.StatusOK {
		t.Fatalf("delete again status=%d body=%q", res.Code, res.Body.String())
	}
	destroy := request(registry, http.MethodDelete, "/users/2/destroy", nil)
	if destroy.Code != http.StatusOK {
		t.Fatalf("destroy status=%d body=%q", destroy.Code, destroy.Body.String())
	}
	withDeleted, err := db.Use[storeUser]().WithDeleted().Where("ID", 2).First(t.Context())
	if err != nil {
		t.Fatalf("query destroyed: %v", err)
	}
	if withDeleted != nil {
		t.Fatalf("destroyed user exists: %#v", withDeleted)
	}
}

func TestOroStoreExportAndScroll(t *testing.T) {
	db := newTestDB(t)
	seedUsers(t, db)

	registry := route.New()
	users := resource.New(route.NewGroup(registry, ""), "/users").Name("user")
	crud.New[storeUser](users, Store[storeUser](db)).
		Actions(crud.ListAction).
		Scroll(1, 2).
		Sort("id", "asc").
		Export[storeUserOutput](func(c *crud.Context[storeUser], model *storeUser) (storeUserOutput, error) {
		return transformStoreUser(c, model), nil
	}, func(exporter *crud.Exporter[storeUser, storeUserOutput]) error {
		exporter.Name("users").Formats("csv").Batch(1)
		exporter.Field("id").Title("ID")
		exporter.Field("name").Title("Name")
		return nil
	}).
		Transform[storeUserOutput](transformStoreUser)

	scroll := request(registry, http.MethodGet, "/users?limit=1", nil)
	var scrollBody struct {
		Items []storeUserOutput `json:"items"`
		Meta  core.Map          `json:"meta"`
	}
	decode(t, scroll, &scrollBody)
	if len(scrollBody.Items) != 1 || scrollBody.Meta["next"] == "" {
		t.Fatalf("scroll = %#v", scrollBody)
	}

	exported := request(registry, http.MethodGet, "/users/export", nil)
	if exported.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%q", exported.Code, exported.Body.String())
	}
	reader := csv.NewReader(bytes.NewReader(exported.Body.Bytes()))
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	if len(rows) != 3 || rows[0][0] != "ID" || rows[1][1] != "alice" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestOroStoreMissingKeyDoesNotDeleteAllRows(t *testing.T) {
	db := newTestDB(t)
	seedUsers(t, db)

	registry := route.New()
	users := resource.New(route.NewGroup(registry, ""), "/users").Name("user")
	crud.New[storeUser](users, Store[storeUser](db)).
		Actions(crud.DeleteAction).
		Transform[storeUserOutput](transformStoreUser)

	response := request(registry, http.MethodDelete, "/users/", nil)
	if response.Code == http.StatusOK {
		t.Fatalf("expected error, body=%q", response.Body.String())
	}
	total, err := db.Use[storeUser]().Count(t.Context())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d", total)
	}
}

func TestOroStoreLikeEscapesWildcards(t *testing.T) {
	db := newTestDB(t)
	seedUsers(t, db)
	role, err := db.Use[storeRole]().Where("Name", "admin").First(t.Context())
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	if _, err := db.Use[storeUser]().Create(t.Context(), &storeUser{Name: "100%_ok", Email: "literal@example.com", Status: 1, RoleID: role.ID}); err != nil {
		t.Fatalf("create literal: %v", err)
	}
	if _, err := db.Use[storeUser]().Create(t.Context(), &storeUser{Name: "100xxok", Email: "wildcard@example.com", Status: 1, RoleID: role.ID}); err != nil {
		t.Fatalf("create wildcard: %v", err)
	}

	registry := route.New()
	users := resource.New(route.NewGroup(registry, ""), "/users").Name("user")
	crud.New[storeUser](users, Store[storeUser](db)).
		Actions(crud.ListAction).
		Filters(filter.Like("name")).
		Transform[storeUserOutput](transformStoreUser)

	like := request(registry, http.MethodGet, "/users?name=100%25_ok", nil)
	var likeBody struct {
		Items []storeUserOutput `json:"items"`
		Meta  core.Map          `json:"meta"`
	}
	decode(t, like, &likeBody)
	if len(likeBody.Items) != 1 || likeBody.Items[0].Name != "100%_ok" {
		t.Fatalf("like body = %#v", likeBody)
	}
}

func formatStoreUser(c *crud.Context[storeUser], f *crud.Formatter[storeUser]) {
	f.Field("name").To(&c.Model.Name)
	f.Field("email").To(&c.Model.Email)
	f.Field("status").To(&c.Model.Status).Actions(crud.CreateAction, crud.EditAction, crud.StoreAction)
	f.Field("role_id").To(&c.Model.RoleID)
}

func transformStoreUser(c *crud.Context[storeUser], user *storeUser) storeUserOutput {
	output := storeUserOutput{
		ID:     user.ID,
		Name:   user.Name,
		Email:  user.Email,
		Status: user.Status,
	}
	role, err := user.Role().One[storeRole]()
	if err == nil && role != nil {
		output.Role = role.Name
	}
	return output
}

func newTestDB(t *testing.T) *oro.DB {
	t.Helper()
	db, err := oro.Open(oro.Config{
		Default: "default",
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatalf("open oro: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(t.Context()); err != nil {
			t.Fatalf("close oro: %v", err)
		}
	})
	if err := db.Register(storeUser{}, storeRole{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Sync(t.Context()); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return db
}

func seedUsers(t *testing.T, db *oro.DB) {
	t.Helper()
	admin, err := db.Use[storeRole]().Create(t.Context(), &storeRole{Name: "admin"})
	if err != nil {
		t.Fatalf("create role admin: %v", err)
	}
	guest, err := db.Use[storeRole]().Create(t.Context(), &storeRole{Name: "guest"})
	if err != nil {
		t.Fatalf("create role guest: %v", err)
	}
	if _, err := db.Use[storeUser]().Create(t.Context(), &storeUser{Name: "alice", Email: "alice@example.com", Status: 1, RoleID: admin.ID}); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := db.Use[storeUser]().Create(t.Context(), &storeUser{Name: "bob", Email: "bob@example.com", Status: 2, RoleID: guest.ID}); err != nil {
		t.Fatalf("create bob: %v", err)
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
