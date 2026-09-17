package web

import (
	"compress/gzip"
	"io"
	"io/fs"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

func TestStaticCompressionAndCacheValidation(t *testing.T) {
	data := []byte(strings.Repeat("console.log('hello');", 100))
	assets := fstest.MapFS{"assets/app-abcdefgh.js": &fstest.MapFile{Data: data}, "index.html": &fstest.MapFile{Data: data}, "assets/stardew/ui/test.png": &fstest.MapFile{Data: data}}
	cache := &staticContentCache{assets: assets}
	get := func(path, encoding, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/"+path, nil)
		r.Header.Set("Accept-Encoding", encoding)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		if err := cache.serve(w, r, path); err != nil {
			t.Fatal(err)
		}
		return w
	}
	gz := get("assets/app-abcdefgh.js", "br, gzip", "")
	if gz.Code != 200 || gz.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(gz.Header().Get("Cache-Control"), "immutable") {
		t.Fatal(gz.Result())
	}
	reader, err := gzip.NewReader(gz.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != string(data) {
		t.Fatal("gzip did not preserve asset")
	}
	_ = reader.Close()
	if got := get("assets/app-abcdefgh.js", "gzip", gz.Header().Get("ETag")); got.Code != 304 || got.Body.Len() != 0 {
		t.Fatal("conditional GET did not return 304")
	}
	plain := get("assets/app-abcdefgh.js", "gzip;q=0", "")
	if plain.Header().Get("Content-Encoding") != "" || plain.Body.String() != string(data) || plain.Header().Get("ETag") == gz.Header().Get("ETag") {
		t.Fatal("encoding negotiation/validator mismatch")
	}
	for _, path := range []string{"index.html", "assets/stardew/ui/test.png"} {
		got := get(path, "", "")
		if got.Header().Get("Cache-Control") != "no-cache" || got.Header().Get("ETag") == "" {
			t.Fatal("unversioned asset must revalidate")
		}
	}
}

type countedStaticFS struct {
	fs.FS
	reads atomic.Int64
}

func (f *countedStaticFS) ReadFile(name string) ([]byte, error) {
	f.reads.Add(1)
	return fs.ReadFile(f.FS, name)
}

func TestStaticCacheReadsOnceAndSeparatesBuilds(t *testing.T) {
	const name = "assets/app-abcdefgh.js"
	data := []byte(strings.Repeat("console.log('hello');", 100))
	files := &countedStaticFS{FS: fstest.MapFS{name: &fstest.MapFile{Data: data}}}
	cache := &staticContentCache{assets: files}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/"+name, nil)
			r.Header.Set("Accept-Encoding", "gzip")
			if err := cache.serve(httptest.NewRecorder(), r, name); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if files.reads.Load() != 2 {
		t.Fatalf("concurrent cold reads: %d, want one original + one gzip probe", files.reads.Load())
	}
	plain, err := cache.representation(name, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/"+name, nil)
		r.Header.Set("If-None-Match", plain.etag)
		w := httptest.NewRecorder()
		if err := cache.serve(w, r, name); err != nil || w.Code != 304 || w.Body.Len() != 0 {
			t.Fatalf("conditional %s: %d %v", method, w.Code, err)
		}
	}
	r := httptest.NewRequest("GET", "/"+name, nil)
	r.Header.Set("Range", "bytes=0-6")
	w := httptest.NewRecorder()
	if err := cache.serve(w, r, name); err != nil || w.Code != 206 || w.Body.String() != string(data[:7]) {
		t.Fatal("range failed")
	}
	if files.reads.Load() != 2 {
		t.Fatal("warm HEAD/304/range reread assets")
	}
	other := &staticContentCache{assets: fstest.MapFS{name: &fstest.MapFile{Data: []byte("new build")}}}
	next, err := other.representation(name, false)
	if err != nil || next.etag == plain.etag || string(next.data) != "new build" {
		t.Fatal("cache crossed build boundary")
	}
	missing := &staticContentCache{assets: fstest.MapFS{}}
	for i := 0; i < 50; i++ {
		_, _ = missing.representation(strings.Repeat("x", i+1)+".js", true)
	}
	missing.entries.Range(func(_, _ any) bool { t.Error("missing paths retained"); return false })
}

func BenchmarkStaticConditionalAsset(b *testing.B) {
	const name = "assets/stardew/ui/background.webp"
	cache := &staticContentCache{assets: fstest.MapFS{name: &fstest.MapFile{Data: []byte(strings.Repeat("x", 1564122))}}}
	value, err := cache.representation(name, false)
	if err != nil {
		b.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/"+name, nil)
	r.Header.Set("If-None-Match", value.etag)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		if err := cache.serve(w, r, name); err != nil || w.Code != 304 {
			b.Fatal(err, w.Code)
		}
	}
}
