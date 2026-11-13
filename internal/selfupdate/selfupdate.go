// Package selfupdate replaces the running binary with the latest GitHub
// release, after checking it against the release's checksums.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// LatestURL is the GitHub API endpoint for the latest release.
const LatestURL = "https://api.github.com/repos/notzenco/wordle-solver/releases/latest"

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// parseVersion reads a release version such as v1.2.3. Development builds
// ("dev", or a Go pseudo-version) are not release versions.
func parseVersion(v string) (parts [3]int, ok bool) {
	fields := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if !strings.HasPrefix(v, "v") || len(fields) != 3 {
		return parts, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// newerVersion reports whether latest is a later release than current; ok
// is false when either is not a release version.
func newerVersion(current, latest string) (newer, ok bool) {
	c, ok1 := parseVersion(current)
	l, ok2 := parseVersion(latest)
	if !ok1 || !ok2 {
		return false, false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i], true
		}
	}
	return false, true
}

// assetName is the release archive for a platform, as
// scripts/build-release.sh names it.
func assetName(tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("wordle-solver_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext)
}

// Update replaces exe with the latest release for goos/goarch when it is
// newer than current, and returns the version installed, or "" when exe is
// already up to date. A development build is replaced only with force.
func Update(ctx context.Context, apiURL, current, exe, goos, goarch string, force bool) (string, error) {
	var rel release
	body, err := get(ctx, apiURL)
	if errors.Is(err, errNotFound) {
		return "", errors.New("no release has been published yet")
	}
	if err != nil {
		return "", fmt.Errorf("looking up the latest release: %w", err)
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("reading the latest release: %w", err)
	}
	newer, ok := newerVersion(current, rel.Tag)
	switch {
	case !ok && !force:
		return "", fmt.Errorf("this is a development build (%s); 'update -force' replaces it with release %s", current, rel.Tag)
	case ok && !newer && !force:
		return "", nil
	}

	name := assetName(rel.Tag, goos, goarch)
	urls := map[string]string{}
	for _, a := range rel.Assets {
		urls[a.Name] = a.URL
	}
	if urls[name] == "" || urls["checksums.txt"] == "" {
		return "", fmt.Errorf("release %s has no build for %s/%s", rel.Tag, goos, goarch)
	}
	archive, err := get(ctx, urls[name])
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", name, err)
	}
	sums, err := get(ctx, urls["checksums.txt"])
	if err != nil {
		return "", fmt.Errorf("downloading checksums: %w", err)
	}
	if err := verifyChecksum(archive, name, sums); err != nil {
		return "", err
	}
	binary, err := extractBinary(archive, goos)
	if err != nil {
		return "", fmt.Errorf("unpacking %s: %w", name, err)
	}
	if err := replaceExe(exe, binary); err != nil {
		return "", err
	}
	return rel.Tag, nil
}

var errNotFound = errors.New("not found")

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "wordle-solver")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
}

func verifyChecksum(data []byte, name string, sums []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == name {
			sum := sha256.Sum256(data)
			if fields[0] != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("%s does not match its checksum; not installing it", name)
			}
			return nil
		}
	}
	return fmt.Errorf("checksums.txt does not list %s", name)
}

// extractBinary returns the wordle-solver executable inside a release archive.
func extractBinary(archive []byte, goos string) ([]byte, error) {
	if goos == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if path.Base(f.Name) == "wordle-solver.exe" {
				r, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer r.Close()
				return io.ReadAll(r)
			}
		}
		return nil, errors.New("no wordle-solver.exe inside")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("no wordle-solver inside")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == "wordle-solver" {
			return io.ReadAll(tr)
		}
	}
}

// replaceExe swaps in the new executable with renames in exe's folder, so
// exe is never left half written. Windows will not overwrite a running
// program but does let it be renamed, so the old one is moved aside to
// exe.old and deleted on a later run.
func replaceExe(exe string, binary []byte) error {
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, binary, 0o755); err != nil {
		return fmt.Errorf("can't write next to %s (%w); rerun with permission to change it", exe, err)
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Rename(old, exe)
		os.Remove(tmp)
		return err
	}
	// A running Windows program can't be deleted; RemoveOld tidies up later.
	os.Remove(old)
	return nil
}

// RemoveOld deletes the copy of exe that an update on Windows left aside.
func RemoveOld(exe string) {
	os.Remove(exe + ".old")
}
