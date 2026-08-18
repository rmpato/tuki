package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease serves a GitHub-shaped release with real archives, so the tests
// exercise the same download, verify, and unpack path as the real thing.
type fakeRelease struct {
	*httptest.Server
	archives map[string][]byte
	sums     []byte
	tag      string
}

func newFakeRelease(t *testing.T, tag string, binary []byte) *fakeRelease {
	t.Helper()

	f := &fakeRelease{archives: map[string][]byte{}, tag: tag}

	for _, p := range []struct{ goos, goarch string }{
		{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"},
	} {
		f.archives[AssetName(tag, p.goos, p.goarch)] = tarGz(t, "tuki", binary)
	}

	var sums strings.Builder
	for name, data := range f.archives {
		sum := sha256.Sum256(data)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	f.sums = []byte(sums.String())

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("requests to GitHub must set a User-Agent")
		}
		var assets []string
		for name := range f.archives {
			assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`,
				name, f.URL+"/download/"+name))
		}
		assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`,
			checksumsAsset, f.URL+"/download/"+checksumsAsset))

		fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example.test/rel","body":"a note\n\nanother note","assets":[%s]}`,
			f.tag, strings.Join(assets, ","))
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/download/")
		if name == checksumsAsset {
			w.Write(f.sums)
			return
		}
		data, ok := f.archives[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})

	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeRelease) client() *Client {
	return &Client{HTTP: f.Server.Client(), API: f.URL}
}

func tarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// A real archive carries the docs alongside the binary.
	for _, f := range []struct {
		name    string
		content []byte
	}{
		{"README.md", []byte("# tuki")},
		{name, content},
		{"LICENSE", []byte("MIT")},
	} {
		if err := tw.WriteHeader(&tar.Header{
			Name: f.name, Mode: 0o755, Size: int64(len(f.content)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLatestReadsTheRelease(t *testing.T) {
	f := newFakeRelease(t, "v0.9.0", []byte("binary"))

	rel, err := f.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "v0.9.0" {
		t.Errorf("Version = %q", rel.Version)
	}
	if rel.URL != "https://example.test/rel" {
		t.Errorf("URL = %q", rel.URL)
	}
	if !strings.Contains(rel.Notes, "another note") {
		t.Errorf("Notes = %q", rel.Notes)
	}
}

func TestBinaryDownloadsAndUnpacks(t *testing.T) {
	want := []byte("the actual tuki binary")
	f := newFakeRelease(t, "v0.9.0", want)

	rel, err := f.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.client().Binary(context.Background(), rel, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted %q, want %q", got, want)
	}
}

// The whole point of the checksum file: a tampered archive must not install.
func TestTamperedDownloadIsRejected(t *testing.T) {
	f := newFakeRelease(t, "v0.9.0", []byte("the real binary"))

	rel, err := f.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Swap the archive contents after the checksums were published.
	name := AssetName("v0.9.0", "darwin", "arm64")
	f.archives[name] = tarGz(t, "tuki", []byte("something else entirely"))

	_, err = f.client().Binary(context.Background(), rel, "darwin", "arm64")
	if err == nil {
		t.Fatal("a mismatched checksum should have stopped the update")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error should explain the checksum failure, got: %v", err)
	}
}

func TestMissingChecksumsIsRejected(t *testing.T) {
	f := newFakeRelease(t, "v0.9.0", []byte("binary"))
	rel, err := f.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	delete(rel.assets, checksumsAsset)

	if _, err := f.client().Binary(context.Background(), rel, "darwin", "arm64"); err == nil {
		t.Error("without a checksums file the update should refuse to proceed")
	}
}

func TestUnsupportedPlatformIsReported(t *testing.T) {
	f := newFakeRelease(t, "v0.9.0", []byte("binary"))
	rel, err := f.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.client().Binary(context.Background(), rel, "plan9", "mips")
	if err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Errorf("expected a clear per-platform error, got: %v", err)
	}
}

func TestAssetNameMatchesGoReleaser(t *testing.T) {
	cases := map[string]string{
		"darwin/arm64":  "tuki_1.2.3_darwin_arm64.tar.gz",
		"linux/amd64":   "tuki_1.2.3_linux_amd64.tar.gz",
		"windows/amd64": "tuki_1.2.3_windows_amd64.zip",
	}
	for platform, want := range cases {
		parts := strings.Split(platform, "/")
		if got := AssetName("v1.2.3", parts[0], parts[1]); got != want {
			t.Errorf("AssetName(%s) = %q, want %q", platform, got, want)
		}
	}
}

func TestReplaceSwapsTheBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tuki")
	if err := os.WriteFile(path, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(path, []byte("new version")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new version" {
		t.Errorf("file contains %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no executable bit — everything there is -rw-rw-rw- and
	// runnability comes from the extension instead.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the new binary should be executable, mode is %v", info.Mode())
	}

	// Nothing should be left lying around next to it.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want just the binary", names)
	}
}

func TestReplaceRefusesEmptyBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tuki")
	if err := os.WriteFile(path, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(path, nil); err == nil {
		t.Error("replacing with an empty binary should fail")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old version" {
		t.Error("the existing binary should have been left alone")
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.1.0", "v0.2.0", -1},
		{"v0.2.0", "v0.1.0", 1},
		{"v1.0.0", "v1.0.0", 0},
		{"0.1.0", "v0.1.0", 0},
		{"v0.9.0", "v0.10.0", -1}, // not string ordering
		{"v1.0.0", "v1.0.1", -1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.0.0-rc1", "v1.0.0", -1}, // a pre-release precedes its release
		{"v1.0.0", "v1.0.0-rc1", 1},
		{"dev", "v0.1.0", -1}, // an unparseable build is always behind
		{"dev", "dev", 0},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v0.1.0", "v0.2.0") {
		t.Error("v0.2.0 should be newer than v0.1.0")
	}
	if IsNewer("v0.2.0", "v0.2.0") {
		t.Error("the same version is not newer")
	}
	if IsNewer("v0.3.0", "v0.2.0") {
		t.Error("an older release should not count as an update")
	}
	if !IsNewer("dev", "v0.1.0") {
		t.Error("a dev build should be offered the latest release")
	}
}

func TestChecksumForHandlesBinaryMarker(t *testing.T) {
	sums := []byte("abc123  tuki_1.0.0_linux_amd64.tar.gz\ndef456 *tuki_1.0.0_darwin_arm64.tar.gz\n")

	if got, ok := checksumFor(sums, "tuki_1.0.0_linux_amd64.tar.gz"); !ok || got != "abc123" {
		t.Errorf("plain entry = %q, %v", got, ok)
	}
	if got, ok := checksumFor(sums, "tuki_1.0.0_darwin_arm64.tar.gz"); !ok || got != "def456" {
		t.Errorf("starred entry = %q, %v", got, ok)
	}
	if _, ok := checksumFor(sums, "nope.tar.gz"); ok {
		t.Error("an absent file should not resolve")
	}
}
