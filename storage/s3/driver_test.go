package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/duxweb/runa"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/storage"
)

func TestDriverNameAndURL(t *testing.T) {
	driver := Driver(Bucket("bucket"), Region("us-east-1"), Endpoint("http://127.0.0.1:9000"), PathStyle(true)).(*driver)
	if driver.Name() != "s3" {
		t.Fatalf("name = %q", driver.Name())
	}
	url, err := driver.URL(nil, "dir/file.txt", storage.URLOptions{})
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if !strings.Contains(url, "bucket/dir/file.txt") {
		t.Fatalf("url = %q", url)
	}
}

func TestDriverUsesDefaultAWSRegionWhenUnset(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	driver := Driver(Bucket("bucket")).(*driver)
	if driver.client == nil {
		t.Fatal("client is nil")
	}
	if driver.options.region != "" {
		t.Fatalf("driver option region should stay unset for AWS default chain, got %q", driver.options.region)
	}
}

func TestDriverDisablesSupportedChecksumsForCustomEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	driver := Driver(Bucket("bucket"), Endpoint("https://oss-cn-hangzhou.aliyuncs.com")).(*driver)
	if driver.client == nil {
		t.Fatal("client is nil")
	}
	if driver.options.endpoint == "" {
		t.Fatal("endpoint is empty")
	}
	if got := driver.client.Options().RequestChecksumCalculation; got != aws.RequestChecksumCalculationWhenRequired {
		t.Fatalf("checksum calculation = %v", got)
	}
}

func TestCopyEncodesCopySource(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	var copySource string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		copySource = request.Header.Get("X-Amz-Copy-Source")
		writer.Header().Set("Content-Type", "application/xml")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`<CopyObjectResult><LastModified>2026-07-03T00:00:00.000Z</LastModified><ETag>"etag"</ETag></CopyObjectResult>`))
	}))
	defer server.Close()

	driver := Driver(
		Bucket("bucket"),
		Region("us-east-1"),
		Endpoint(server.URL),
		PathStyle(true),
	).(*driver)
	if err := driver.Copy(context.Background(), "目录/中文 file+1#.txt", "目标.txt", storage.FileOptions{}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	want := "bucket/%E7%9B%AE%E5%BD%95/%E4%B8%AD%E6%96%87%20file%2B1%23.txt"
	if copySource != want {
		t.Fatalf("copy source = %q, want %q", copySource, want)
	}
}

func TestCustomEndpointDeleteUsesSingleObjectRequests(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Fatalf("method = %s", request.Method)
		}
		if request.URL.Query().Has("delete") {
			t.Fatal("custom endpoint should not use DeleteObjects")
		}
		paths = append(paths, request.URL.EscapedPath())
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	driver := Driver(
		Bucket("bucket"),
		Region("us-east-1"),
		Endpoint(server.URL),
		PathStyle(true),
	).(*driver)
	if err := driver.Delete(context.Background(), "a.txt", "目录/中文 file+1#.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/bucket/a.txt" || paths[1] != "/bucket/%E7%9B%AE%E5%BD%95/%E4%B8%AD%E6%96%87%20file%2B1%23.txt" {
		t.Fatalf("delete paths = %#v", paths)
	}
}

func TestProviderConfigResolution(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "s3.toml", "bucket = 'shared'\nregion = 'us-east-1'\nendpoint = 'https://shared.example.com'\npath_style = false\n")
	writeConfig(t, root, "storage.toml", "[s3]\nbucket = 'feature'\nendpoint = 'https://feature.example.com'\npath_style = true\ndomain = 'https://cdn.example.com'\nurl_prefix = '/uploads'\n")

	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(storage.Provider(), Provider(Client(s3.New(s3.Options{}))))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	registered := storage.Default().Driver("s3")
	driver, ok := registered.(*driver)
	if !ok {
		t.Fatalf("registered driver = %T", registered)
	}
	if driver.options.bucket != "feature" {
		t.Fatalf("bucket = %q", driver.options.bucket)
	}
	if driver.options.region != "us-east-1" {
		t.Fatalf("region = %q", driver.options.region)
	}
	if driver.options.endpoint != "https://feature.example.com" || !driver.options.pathStyle {
		t.Fatalf("endpoint/path_style = %q/%v", driver.options.endpoint, driver.options.pathStyle)
	}
	url, err := driver.URL(context.Background(), "avatars/1.png", storage.URLOptions{})
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if url != "https://cdn.example.com/uploads/avatars/1.png" {
		t.Fatalf("url = %q", url)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderNamedSharedConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "s3.toml", "[archive]\nbucket = 'archive'\nregion = 'eu-central-1'\nendpoint = 'https://archive.example.com'\n")

	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(storage.Provider(), Provider(Use("archive"), Client(s3.New(s3.Options{}))))
	app.Module(providerTestModule{})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	driver := storage.Default().Driver("s3").(*driver)
	if driver.options.bucket != "archive" || driver.options.region != "eu-central-1" || driver.options.endpoint != "https://archive.example.com" {
		t.Fatalf("unexpected options: %+v", driver.options)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	path := filepath.Join(root, "config")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

type providerTestModule struct{}

func (providerTestModule) Name() string { return "storage-s3-provider-test" }
func (providerTestModule) Init(context.Context, runaprovider.Context) error {
	return nil
}
func (providerTestModule) Register(context.Context, runaprovider.Context) error {
	return nil
}
func (providerTestModule) Boot(context.Context, runaprovider.Context) error {
	return nil
}
func (providerTestModule) Shutdown(context.Context, runaprovider.Context) error {
	return nil
}
