package storage

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalDiskReadWriteAndFileOps(t *testing.T) {
	root := t.TempDir()
	registry := New()
	registry.RegisterDriver(DefaultDriver, LocalDriver(Root(root), DriverURLPrefix("/files")))
	disk := registry.MustOf(DiskPublic)
	ctx := context.Background()

	if err := disk.PutString(ctx, "avatars/1.txt", "hello", ContentType("text/plain")); err != nil {
		t.Fatalf("put string: %v", err)
	}
	body, err := disk.GetString(ctx, "avatars/1.txt")
	if err != nil {
		t.Fatalf("get string: %v", err)
	}
	if body != "hello" {
		t.Fatalf("unexpected body %q", body)
	}
	exists, err := disk.Exists(ctx, "avatars/1.txt")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	size, err := disk.Size(ctx, "avatars/1.txt")
	if err != nil || size != 5 {
		t.Fatalf("size=%d err=%v", size, err)
	}
	info, err := disk.Info(ctx, "avatars/1.txt")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Path != "avatars/1.txt" || info.Size != 5 {
		t.Fatalf("unexpected info: %+v", info)
	}
	if err := disk.Copy(ctx, "avatars/1.txt", "avatars/2.txt"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	copied, _ := disk.GetString(ctx, "avatars/2.txt")
	if copied != "hello" {
		t.Fatalf("unexpected copied body %q", copied)
	}
	if err := disk.Move(ctx, "avatars/2.txt", "avatars/3.txt"); err != nil {
		t.Fatalf("move: %v", err)
	}
	exists, _ = disk.Exists(ctx, "avatars/2.txt")
	if exists {
		t.Fatal("source should be deleted after move")
	}
	moved, _ := disk.GetString(ctx, "avatars/3.txt")
	if moved != "hello" {
		t.Fatalf("unexpected moved body %q", moved)
	}
	if err := disk.Delete(ctx, "avatars/1.txt", "avatars/3.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	exists, _ = disk.Exists(ctx, "avatars/1.txt")
	if exists {
		t.Fatal("file should be deleted")
	}
}

func TestLocalDiskList(t *testing.T) {
	root := t.TempDir()
	registry := New(Root(root))
	registry.Disk("docs", Use(DefaultDriver), Prefix("docs"))
	disk := registry.MustOf("docs")
	ctx := context.Background()

	files := map[string]string{
		"2026/a.txt":        "a",
		"2026/b.txt":        "b",
		"2026/nested/c.txt": "c",
		"2025/old.txt":      "old",
	}
	for name, body := range files {
		if err := disk.PutString(ctx, name, body); err != nil {
			t.Fatalf("put %s: %v", name, err)
		}
	}

	page, err := disk.List(ctx, "2026", Limit(2), Recursive())
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page.Items) != 2 || !page.HasMore || page.Cursor == "" {
		t.Fatalf("unexpected page 1: %+v", page)
	}
	if page.Items[0].Path != "2026/a.txt" || page.Items[1].Path != "2026/b.txt" {
		t.Fatalf("unexpected page 1 items: %+v", page.Items)
	}

	next, err := disk.List(ctx, "2026", Limit(2), Recursive(), Cursor(page.Cursor))
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(next.Items) != 1 || next.Items[0].Path != "2026/nested/c.txt" || next.HasMore {
		t.Fatalf("unexpected page 2: %+v", next)
	}

	flat, err := disk.List(ctx, "2026")
	if err != nil {
		t.Fatalf("list flat: %v", err)
	}
	if len(flat.Items) != 2 || len(flat.CommonDirs) != 1 || flat.CommonDirs[0] != "2026/nested" {
		t.Fatalf("unexpected flat list: %+v", flat)
	}
}

func TestNewCreatesDirectLocalDisk(t *testing.T) {
	disk := New(Root(t.TempDir())).MustOf(DiskLocal)
	ctx := context.Background()
	if err := disk.PutString(ctx, "hello.txt", "world"); err != nil {
		t.Fatalf("put: %v", err)
	}
	value, err := disk.GetString(ctx, "hello.txt")
	if err != nil || value != "world" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}

func TestLocalDiskRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	driver := LocalDriver(Root(root))
	registry := New()
	registry.RegisterDriver(DefaultDriver, driver)
	disk := registry.MustOf(DiskLocal)

	if err := disk.PutString(context.Background(), "../escape.txt", "bad"); err == nil {
		t.Fatal("expected path traversal error")
	}
	if _, err := os.Stat(root + "/../escape.txt"); err == nil {
		t.Fatal("escaped file should not be written")
	}
}

func TestLocalDiskRejectsSymlinkTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, root+"/link"); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	driver := LocalDriver(Root(root))
	registry := New()
	registry.RegisterDriver(DefaultDriver, driver)
	disk := registry.MustOf(DiskLocal)

	if err := disk.PutString(context.Background(), "link/escape.txt", "bad"); err == nil {
		t.Fatal("expected symlink traversal error")
	}
	if _, err := os.Stat(outside + "/escape.txt"); err == nil {
		t.Fatal("escaped file should not be written through symlink")
	}
}

func TestLocalURLAndSignedURLs(t *testing.T) {
	registry := New()
	registry.RegisterDriver(DefaultDriver, LocalDriver(Root(t.TempDir()), DriverDomain("https://cdn.example.com"), DriverURLPrefix("/uploads"), Secret("secret")))
	registry.Disk("cdn", Use(DefaultDriver), Prefix("public"), Public(), URLPrefix("/assets"))
	disk := registry.MustOf("cdn")
	ctx := context.Background()

	urlValue, err := disk.URL(ctx, "avatar.png")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if urlValue != "https://cdn.example.com/assets/public/avatar.png" {
		t.Fatalf("unexpected url %q", urlValue)
	}
	temp, err := disk.TempURL(ctx, "avatar.png", time.Minute)
	if err != nil {
		t.Fatalf("temp url: %v", err)
	}
	parsed, err := url.Parse(temp)
	if err != nil {
		t.Fatalf("parse temp url: %v", err)
	}
	if parsed.Query().Get("signature") == "" || parsed.Query().Get("expires") == "" {
		t.Fatalf("temp url missing signature params: %s", temp)
	}
	signedPut, err := disk.SignPut(ctx, "avatar.png", time.Minute, ContentType("image/png"))
	if err != nil {
		t.Fatalf("sign put: %v", err)
	}
	if signedPut.Method != http.MethodPut || signedPut.Header["Content-Type"] != "image/png" || signedPut.URL == "" {
		t.Fatalf("unexpected signed put: %+v", signedPut)
	}
	signedPost, err := disk.SignPost(ctx, "avatar.png", time.Minute, ContentType("image/png"))
	if err != nil {
		t.Fatalf("sign post: %v", err)
	}
	if signedPost.Fields["key"] != "public/avatar.png" || signedPost.Fields["signature"] == "" {
		t.Fatalf("unexpected signed post: %+v", signedPost)
	}
}

func TestLocalSignedURLsRequireSecret(t *testing.T) {
	registry := New()
	registry.RegisterDriver(DefaultDriver, LocalDriver(Root(t.TempDir()), DriverDomain("https://cdn.example.com")))
	disk := registry.MustOf(DiskPublic)
	ctx := context.Background()

	if _, err := disk.TempURL(ctx, "avatar.png", time.Minute); err == nil {
		t.Fatal("expected temp url secret error")
	}
	if _, err := disk.SignPut(ctx, "avatar.png", time.Minute); err == nil {
		t.Fatal("expected sign put secret error")
	}
	if _, err := disk.SignPost(ctx, "avatar.png", time.Minute); err == nil {
		t.Fatal("expected sign post secret error")
	}
	if urlValue, err := disk.TempURL(ctx, "avatar.png", 0); err != nil || urlValue == "" {
		t.Fatalf("plain temp url = %q err=%v", urlValue, err)
	}
}

func TestRegistryExternalDriverAndInfo(t *testing.T) {
	registry := New()
	driver := &fakeDriver{name: "fake"}
	registry.RegisterDriver("fake", driver)
	registry.Disk("docs", Use("fake"), Prefix("docs"), Public(), Meta("module", "test"))
	disk, err := registry.Of("docs")
	if err != nil {
		t.Fatalf("of docs: %v", err)
	}
	if err := disk.PutString(context.Background(), "a.txt", "body"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if driver.lastPath != "docs/a.txt" {
		t.Fatalf("unexpected driver path %q", driver.lastPath)
	}
	infos := registry.Info()
	var found Info
	for _, item := range infos {
		if item.Name == "docs" {
			found = item
		}
	}
	if found.Name != "docs" || found.Driver != "fake" || !found.Public || found.Meta["module"] != "test" {
		t.Fatalf("unexpected info: %+v", found)
	}
}

type fakeDriver struct {
	name     string
	lastPath string
}

func (driver *fakeDriver) Name() string { return driver.name }
func (driver *fakeDriver) Put(_ context.Context, path string, body io.Reader, _ FileOptions) error {
	driver.lastPath = path
	_, _ = io.ReadAll(body)
	return nil
}
func (driver *fakeDriver) Get(context.Context, string) (io.ReadCloser, FileInfo, error) {
	return io.NopCloser(strings.NewReader("")), FileInfo{}, nil
}
func (driver *fakeDriver) Delete(context.Context, ...string) error        { return nil }
func (driver *fakeDriver) Exists(context.Context, string) (bool, error)   { return true, nil }
func (driver *fakeDriver) Info(context.Context, string) (FileInfo, error) { return FileInfo{}, nil }
func (driver *fakeDriver) List(context.Context, string, ListOptions) (FileList, error) {
	return FileList{}, nil
}
func (driver *fakeDriver) URL(_ context.Context, path string, _ URLOptions) (string, error) {
	return "/" + path, nil
}
func (driver *fakeDriver) TempURL(_ context.Context, path string, _ time.Duration, _ URLOptions) (string, error) {
	return "/" + path, nil
}
func (driver *fakeDriver) SignPut(context.Context, string, time.Duration, FileOptions) (SignedURL, error) {
	return SignedURL{}, nil
}
func (driver *fakeDriver) SignPost(context.Context, string, time.Duration, FileOptions) (SignedPost, error) {
	return SignedPost{}, nil
}
func (driver *fakeDriver) Close(context.Context) error { return nil }
