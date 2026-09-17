package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type staticRepresentation struct {
	data        []byte
	etag        string
	gzip        bool
	contentType string
}

// A cache belongs to one immutable embedded build. Replacing the filesystem
// requires a new cache, so paths cannot retain bytes from an earlier build.
type staticContentCache struct {
	assets  fs.FS
	entries sync.Map
}

type staticContentEntry struct {
	once  sync.Once
	value staticRepresentation
	err   error
}

func (c *staticContentCache) representation(name string, compressed bool) (staticRepresentation, error) {
	key := name + "/identity"
	if compressed {
		key = name + "/gzip"
	}
	entryValue, found := c.entries.Load(key)
	if !found {
		entryValue, _ = c.entries.LoadOrStore(key, &staticContentEntry{})
	}
	entry := entryValue.(*staticContentEntry)
	entry.once.Do(func() {
		if compressed {
			plain, err := c.representation(name, false)
			if err != nil {
				entry.err = err
				return
			}
			entry.value.contentType = plain.contentType
			data, err := fs.ReadFile(c.assets, name+".gz")
			if err != nil {
				var buf bytes.Buffer
				writer := gzip.NewWriter(&buf)
				_, _ = writer.Write(plain.data)
				_ = writer.Close()
				data = buf.Bytes()
			}
			entry.value.data, entry.value.gzip = data, true
		} else {
			data, err := fs.ReadFile(c.assets, name)
			if err != nil {
				entry.err = err
				return
			}
			entry.value.data = data
			entry.value.contentType = detectContentType(name, data)
		}
		entry.value.etag = fmt.Sprintf(`"%x"`, sha256.Sum256(entry.value.data))
	})
	if entry.err != nil {
		// Arbitrary missing URLs must not grow a permanent negative cache.
		c.entries.CompareAndDelete(key, entry)
	}
	return entry.value, entry.err
}

var hashedStaticAsset = regexp.MustCompile(`^assets/[^/]+-[A-Za-z0-9_-]{8,}\.(js|css|woff2?|webp|png|svg)$`)

func acceptsGzip(header string) bool {
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		if strings.EqualFold(parts[0], "gzip") {
			for _, param := range parts[1:] {
				key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
				if ok && strings.EqualFold(key, "q") {
					q, err := strconv.ParseFloat(value, 64)
					return err == nil && q > 0 && q <= 1
				}
			}
			return true
		}
	}
	return false
}

func (c *staticContentCache) serve(w http.ResponseWriter, r *http.Request, name string) error {
	compressible := strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") || strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".svg") || strings.HasSuffix(name, ".json")
	compressed := compressible && acceptsGzip(r.Header.Get("Accept-Encoding"))
	representation, err := c.representation(name, compressed)
	if err != nil {
		return err
	}
	if compressible {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	w.Header().Set("Content-Type", representation.contentType)
	w.Header().Set("ETag", representation.etag)
	w.Header().Set("Cache-Control", "no-cache")
	if hashedStaticAsset.MatchString(name) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if representation.gzip {
		w.Header().Set("Content-Encoding", "gzip")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(representation.data))
	return nil
}
