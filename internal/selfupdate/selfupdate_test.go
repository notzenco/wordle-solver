package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.10.0", "v0.9.3", false},
		{"v1.2.3", "v1.2.10", true},
		{"v1.9.9", "v2.0.0", true},
	} {
		if got, ok := newerVersion(c.current, c.latest); !ok || got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, %v; want %v", c.current, c.latest, got, ok, c.want)
		}
	}
	for _, dev := range []string{"dev", "v0.0.0-20261112212706-975ba9534641+dirty"} {
		if _, ok := newerVersion(dev, "v0.2.0"); ok {
			t.Errorf("%q counted as a release version", dev)
		}
	}
}

// fakeRelease serves a GitHub "latest release" API response whose one
// asset is an archive holding binary, for goos/amd64.
func fakeRelease(t *testing.T, tag, goos string, binary []byte, corrupt bool) *httptest.Server {
	t.Helper()
	exe := "wordle-solver"
	if goos == "windows" {
		exe += ".exe"
	}
	name := assetName(tag, goos, "amd64")
	dir := strings.TrimSuffix(strings.TrimSuffix(name, ".zip"), ".tar.gz")
	var archive bytes.Buffer
	if goos == "windows" {
		zw := zip.NewWriter(&archive)
		w, _ := zw.Create(dir + "/" + exe)
		w.Write(binary)
		zw.Close()
	} else {
		gz := gzip.NewWriter(&archive)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: dir + "/README.md", Mode: 0o644, Size: 2})
		tw.Write([]byte("hi"))
		tw.WriteHeader(&tar.Header{Name: dir + "/" + exe, Mode: 0o755, Size: int64(len(binary))})
		tw.Write(binary)
		tw.Close()
		gz.Close()
	}
	sum := sha256.Sum256(archive.Bytes())
	if corrupt {
		sum[0] ^= 1
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"assets": []map[string]string{
				{"name": name, "browser_download_url": srv.URL + "/dl/" + name},
				{"name": "checksums.txt", "browser_download_url": srv.URL + "/dl/checksums.txt"},
			},
		})
	})
	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(archive.Bytes()) })
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%x  other.tar.gz\n%x  %s\n", sha256.Sum256(nil), sum, name)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSelfUpdate(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		srv := fakeRelease(t, "v0.3.0", goos, []byte("new binary"), false)
		exe := filepath.Join(t.TempDir(), "wordle-solver")
		os.WriteFile(exe, []byte("old binary"), 0o755)

		got, err := Update(context.Background(), srv.URL+"/latest", "v0.2.0", exe, goos, "amd64", false)
		if err != nil || got != "v0.3.0" {
			t.Fatalf("%s: selfUpdate = %q, %v", goos, got, err)
		}
		if data, _ := os.ReadFile(exe); string(data) != "new binary" {
			t.Errorf("%s: executable holds %q", goos, data)
		}
		if info, _ := os.Stat(exe); goos != "windows" && info.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s: new executable isn't executable: %v", goos, info.Mode())
		}
	}
}

func TestSelfUpdateUpToDate(t *testing.T) {
	srv := fakeRelease(t, "v0.3.0", "linux", []byte("new binary"), false)
	exe := filepath.Join(t.TempDir(), "wordle-solver")
	os.WriteFile(exe, []byte("old binary"), 0o755)
	got, err := Update(context.Background(), srv.URL+"/latest", "v0.3.0", exe, "linux", "amd64", false)
	if err != nil || got != "" {
		t.Errorf("selfUpdate = %q, %v; want no update", got, err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "old binary" {
		t.Errorf("executable changed to %q", data)
	}
}

func TestSelfUpdateRefusals(t *testing.T) {
	srv := fakeRelease(t, "v0.3.0", "linux", []byte("new binary"), true)
	exe := filepath.Join(t.TempDir(), "wordle-solver")
	os.WriteFile(exe, []byte("old binary"), 0o755)

	if _, err := Update(context.Background(), srv.URL+"/latest", "v0.2.0", exe, "linux", "amd64", false); err == nil ||
		!strings.Contains(err.Error(), "checksum") {
		t.Errorf("corrupt download: err = %v, want a checksum error", err)
	}
	if _, err := Update(context.Background(), srv.URL+"/latest", "dev", exe, "linux", "amd64", false); err == nil {
		t.Error("a development build was replaced without -force")
	}
	if data, _ := os.ReadFile(exe); string(data) != "old binary" {
		t.Errorf("executable changed to %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) != 1 {
		t.Errorf("left files behind: %v", entries)
	}
}

func TestSelfUpdateNoRelease(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := Update(context.Background(), srv.URL, "v0.1.0", "unused", "linux", "amd64", false)
	if err == nil || !strings.Contains(err.Error(), "no release") {
		t.Errorf("err = %v, want a 'no release' error", err)
	}
}
