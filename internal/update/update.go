// Package update checks GitHub for a newer tuki and, when asked to, swaps the
// running binary for it. Everything it downloads is checked against the
// release's own checksum file before it is allowed anywhere near your disk.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultAPI is GitHub's REST endpoint for tuki's releases.
const DefaultAPI = "https://api.github.com/repos/rmpato/tuki"

// maxDownload caps what we're willing to pull down. tuki is a few megabytes;
// anything approaching this is a sign something is wrong.
const maxDownload = 64 << 20

// checksumsAsset is the file GoReleaser publishes alongside the archives.
const checksumsAsset = "checksums.txt"

// Release is the part of a GitHub release that tuki cares about.
type Release struct {
	Version string // as tagged, e.g. "v0.2.0"
	URL     string // the release page, for humans
	Notes   string
	assets  map[string]string // asset name -> download URL
}

// Client talks to GitHub. The zero value is usable.
type Client struct {
	HTTP *http.Client
	API  string // overridden in tests
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) api() string {
	if c.API != "" {
		return c.API
	}
	return DefaultAPI
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub rejects requests without a User-Agent.
	req.Header.Set("User-Agent", "tuki")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

// Latest fetches the most recent published release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	body, err := c.get(ctx, c.api()+"/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("asking GitHub for the latest tuki: %w", err)
	}

	var raw struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("GitHub sent something unexpected: %w", err)
	}
	if raw.TagName == "" {
		return nil, fmt.Errorf("that release has no version tag")
	}

	rel := &Release{
		Version: raw.TagName,
		URL:     raw.HTMLURL,
		Notes:   strings.TrimSpace(raw.Body),
		assets:  make(map[string]string, len(raw.Assets)),
	}
	for _, a := range raw.Assets {
		rel.assets[a.Name] = a.URL
	}
	return rel, nil
}

// AssetName is the archive tuki expects for a given platform. It has to agree
// with the name_template in .goreleaser.yaml.
func AssetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("tuki_%s_%s_%s.%s", strings.TrimPrefix(version, "v"), goos, goarch, ext)
}

// Binary downloads the release archive for a platform, checks it against the
// release's checksum file, and returns the tuki executable inside it.
func (c *Client) Binary(ctx context.Context, rel *Release, goos, goarch string) ([]byte, error) {
	name := AssetName(rel.Version, goos, goarch)

	archiveURL, ok := rel.assets[name]
	if !ok {
		return nil, fmt.Errorf("%s has no build for %s/%s", rel.Version, goos, goarch)
	}
	sumsURL, ok := rel.assets[checksumsAsset]
	if !ok {
		return nil, fmt.Errorf("%s has no %s, so tuki can't verify the download", rel.Version, checksumsAsset)
	}

	archive, err := c.get(ctx, archiveURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", name, err)
	}
	sums, err := c.get(ctx, sumsURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", checksumsAsset, err)
	}

	if err := verify(archive, sums, name); err != nil {
		return nil, err
	}
	return extract(archive, goos)
}

// verify checks the archive against the sha256 recorded for it. A download
// that doesn't match is never unpacked.
func verify(archive, sums []byte, name string) error {
	want, ok := checksumFor(sums, name)
	if !ok {
		return fmt.Errorf("%s isn't listed in %s", name, checksumsAsset)
	}

	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s doesn't match its checksum (got %s, expected %s) — not installing it", name, got, want)
	}
	return nil
}

// checksumFor reads a "sha256␠␠filename" line out of a checksums file.
func checksumFor(sums []byte, name string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// GoReleaser writes "*name" for binary mode in some formats.
		if strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// extract pulls the tuki executable out of a release archive.
func extract(archive []byte, goos string) ([]byte, error) {
	want := "tuki"
	if goos == "windows" {
		want = "tuki.exe"
		return extractZip(archive, want)
	}
	return extractTarGz(archive, want)
}

func extractTarGz(archive []byte, want string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("unpacking the download: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unpacking the download: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != want {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, maxDownload))
	}
	return nil, fmt.Errorf("the download doesn't contain %s", want)
}

func extractZip(archive []byte, want string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("unpacking the download: %w", err)
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("unpacking the download: %w", err)
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, maxDownload))
	}
	return nil, fmt.Errorf("the download doesn't contain %s", want)
}

// ExecutablePath is the file that would be replaced by an update, with any
// symlinks resolved so we swap the real binary rather than the link to it.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("tuki can't tell where it's installed: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// Replace swaps the file at path for new content, keeping the old one aside
// until the new one is safely in place. The current binary is moved rather
// than overwritten, which is what makes this safe on Windows too — and lets
// the whole thing roll back if the second step fails.
func Replace(path string, binary []byte) error {
	if len(binary) == 0 {
		return fmt.Errorf("refusing to install an empty binary")
	}
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".tuki-update-*")
	if err != nil {
		return fmt.Errorf("tuki can't write to %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return fmt.Errorf("writing the new tuki: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing the new tuki: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing the new tuki: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fmt.Errorf("making the new tuki executable: %w", err)
	}

	previous := path + ".old"
	_ = os.Remove(previous) // left over from an earlier update on Windows

	if err := os.Rename(path, previous); err != nil {
		return fmt.Errorf("tuki can't replace %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		// Put the old binary back rather than leaving nothing behind.
		_ = os.Rename(previous, path)
		return fmt.Errorf("tuki can't replace %s: %w", path, err)
	}
	// On Unix this succeeds; on Windows the running image keeps the file
	// locked until exit, which is why it's only best-effort.
	_ = os.Remove(previous)
	return nil
}

// Platform is the current OS and architecture, as GoReleaser names them.
func Platform() (goos, goarch string) { return runtime.GOOS, runtime.GOARCH }

// IsNewer reports whether latest is a later version than current. An
// unparseable current version — "dev", say — counts as older than anything
// released, so a development build is always offered the real thing.
func IsNewer(current, latest string) bool {
	return Compare(current, latest) < 0
}

// Compare orders two versions: -1 if a is older, 0 if equal, 1 if a is newer.
// It understands "v1.2.3" and treats a pre-release suffix as older than the
// release it leads to, which is all tuki's tags ever need.
func Compare(a, b string) int {
	an, apre, aok := parse(a)
	bn, bpre, bok := parse(b)

	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}

	for i := range an {
		if an[i] != bn[i] {
			if an[i] < bn[i] {
				return -1
			}
			return 1
		}
	}

	switch {
	case apre == bpre:
		return 0
	case apre == "": // a release beats its own pre-releases
		return 1
	case bpre == "":
		return -1
	case apre < bpre:
		return -1
	default:
		return 1
	}
}

// parse splits "v1.2.3-rc1" into [1 2 3] and "rc1".
func parse(v string) (nums [3]int, pre string, ok bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nums, "", false
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre, v = v[i+1:], v[:i]
	}

	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return nums, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, "", false
		}
		nums[i] = n
	}
	return nums, pre, true
}
