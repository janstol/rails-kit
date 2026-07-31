package routes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Fingerprint returns an opaque value that changes whenever the route source
// files change: routesRb plus every .rb file (and subdirectory mtime) under
// routesDir, mirroring the invalidation semantics of CacheValid. Stat and
// walk errors, and a missing routesRb or routesDir, are encoded as distinct
// records rather than skipped, so a transient failure never reads as
// "unchanged".
func Fingerprint(routesRb, routesDir string) string {
	type record struct {
		path string
		val  string
	}
	var records []record

	add := func(path, val string) {
		records = append(records, record{filepath.ToSlash(path), val})
	}

	if info, err := os.Stat(routesRb); err != nil {
		add(routesRb, "err:"+err.Error())
	} else {
		add(routesRb, fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size()))
	}

	if dirInfo, err := os.Stat(routesDir); err != nil {
		add(routesDir, "absent")
	} else {
		add(routesDir, fmt.Sprintf("dir:%d", dirInfo.ModTime().UnixNano()))
		_ = filepath.WalkDir(routesDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				add(path, "err:"+err.Error())
				return nil
			}
			if path == routesDir {
				return nil
			}
			if d.IsDir() {
				info, err := d.Info()
				if err != nil {
					add(path, "err:"+err.Error())
					return nil
				}
				add(path, fmt.Sprintf("dir:%d", info.ModTime().UnixNano()))
				return nil
			}
			if !strings.HasSuffix(path, ".rb") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				add(path, "err:"+err.Error())
				return nil
			}
			add(path, fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size()))
			return nil
		})
	}

	sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })

	h := sha256.New()
	for _, r := range records {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", r.path, r.val)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Watch polls the route sources every interval and calls render whenever the
// fingerprint changes. The baseline fingerprint is captured at call time, so
// callers should render once themselves before calling Watch. A render error
// is handed to onErr (if non-nil) and does not stop the loop — a syntax error
// saved into routes.rb, or a failing Rails boot, is exactly when watch mode
// needs to survive. Watch returns nil when ctx is cancelled.
func Watch(ctx context.Context, routesRb, routesDir string, interval time.Duration,
	render func() error, onErr func(error)) error {
	last := Fingerprint(routesRb, routesDir)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fp := Fingerprint(routesRb, routesDir)
			if fp == last {
				continue
			}
			last = fp
			if err := render(); err != nil && onErr != nil {
				onErr(err)
			}
		}
	}
}
