package s3

import (
	"strings"
	"testing"

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
