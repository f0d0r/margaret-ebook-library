package chm

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// countingBlob counts ReadAt calls for memoization tests.
type countingBlob struct {
	book.Blob
	reads *int64
}

func (b countingBlob) ReadAt(p []byte, off int64) (int, error) {
	atomic.AddInt64(b.reads, 1)
	return b.Blob.ReadAt(p, off)
}

func memoFixture() fixture {
	html := `<html><head><title>Doc Title</title>` +
		`<meta name="author" content="Jane Doe">` +
		`</head><body>text</body></html>`
	return fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("Sys Title\x00")},
			{sysDefaultTopic, []byte("/default.html\x00")},
			{sysInfo, sysInfoEntry(1033)},
		}),
		"/default.html": []byte(html),
	}}
}

func TestDefaultTopicMemo(t *testing.T) {
	var reads int64
	cb := countingBlob{book.NewBytesBlob(buildFixture(memoFixture())), &reads}
	c, err := openContainer(cb, chmTestConfig().MaxResourceSize)
	if err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&reads, 0)
	if _, _, err := c.defaultTopicDoc(); err != nil {
		t.Fatalf("defaultTopicDoc: %v", err)
	}
	first := atomic.LoadInt64(&reads)
	if first == 0 {
		t.Fatal("defaultTopicDoc performed no reads")
	}
	if _, _, err := c.defaultTopicDoc(); err != nil {
		t.Fatalf("defaultTopicDoc repeat: %v", err)
	}
	if got := atomic.LoadInt64(&reads); got != first {
		t.Errorf("repeat defaultTopicDoc read %d blobs, want 0", got-first)
	}
	if _, err := c.cachedSystem(); err != nil {
		t.Fatalf("cachedSystem: %v", err)
	}
	if got := atomic.LoadInt64(&reads); got != first {
		t.Errorf("cachedSystem read %d blobs, want 0", got-first)
	}
}

func TestTurnCacheMemo(t *testing.T) {
	plain := []byte("<html><body>cached</body></html>")
	var reads int64
	cb := countingBlob{book.NewBytesBlob(buildFixture(compressedFixture(t, plain))), &reads}
	c, err := openContainer(cb, chmTestConfig().MaxResourceSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.getFile("/content.html"); err != nil {
		t.Fatalf("getFile: %v", err)
	}
	atomic.StoreInt64(&reads, 0)
	if _, err := c.decodeTurn(0, uint64(len(plain))); err != nil {
		t.Fatalf("decodeTurn repeat: %v", err)
	}
	if got := atomic.LoadInt64(&reads); got != 0 {
		t.Errorf("cached decodeTurn read %d blobs, want 0", got)
	}
}

func TestConcurrentReads(t *testing.T) {
	c := openFixture(t, contentFixture())
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths := []string{"/default.html", "/a.html", "/b.html", "/index.hhc", "/#SYSTEM"}
			if _, _, err := c.defaultTopicDoc(); err != nil {
				errs <- err
				return
			}
			_ = c.readMetadata()
			for _, p := range paths {
				if _, err := c.getFile(p); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent read: %v", err)
	}
}

func checkTurnInvariant(t *testing.T, c *container) {
	t.Helper()
	if len(c.turnOrder) > turnCacheMax {
		t.Errorf("turnOrder len %d exceeds cap %d", len(c.turnOrder), turnCacheMax)
	}
	for _, ts := range c.turnOrder {
		if _, ok := c.turns[ts]; !ok {
			t.Errorf("phantom turnOrder key %d missing from map", ts)
		}
	}
}

func TestCacheTurnNoPhantoms(t *testing.T) {
	c := &container{}
	huge := make([]byte, maxCachedTurn+1)
	c.cacheTurn(7, huge)
	if _, ok := c.turns[7]; ok {
		t.Error("oversized turn stored, want skip")
	}
	checkTurnInvariant(t, c)

	for i := uint64(0); i < turnCacheMax+3; i++ {
		c.cacheTurn(i*100, []byte{byte(i)})
		checkTurnInvariant(t, c)
	}
	if len(c.turns) != turnCacheMax || len(c.turnOrder) != turnCacheMax {
		t.Errorf("cache holds %d/%d, want %d/%d", len(c.turns), len(c.turnOrder), turnCacheMax, turnCacheMax)
	}
	if _, ok := c.turns[0]; ok {
		t.Error("evicted turn 0 still cached")
	}

	c.cacheTurn(100, []byte{1, 2, 3})
	if got := c.turns[100]; len(got) != 3 {
		t.Errorf("longer prefix not kept, len %d", len(got))
	}
	c.cacheTurn(100, []byte{1})
	if got := c.turns[100]; len(got) != 3 {
		t.Errorf("shorter prefix replaced longer, len %d", len(got))
	}
	checkTurnInvariant(t, c)
}
