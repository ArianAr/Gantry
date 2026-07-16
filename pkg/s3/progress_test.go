package s3

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestProgressReaderCounts(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefghij"), 1000) // 10_000 bytes
	stats := NewEngineStats("job", "rule")
	pr := NewProgressReader(context.Background(), bytes.NewReader(data), stats, 1, int64(len(data)), 0)

	buf := make([]byte, 1024)
	var total int
	for {
		n, err := pr.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	if total != len(data) {
		t.Fatalf("total=%d want %d", total, len(data))
	}
	if stats.BytesRead.Load() != int64(len(data)) {
		t.Fatalf("stats bytes=%d", stats.BytesRead.Load())
	}
	if pr.BytesRead() != int64(len(data)) {
		t.Fatalf("reader bytes=%d", pr.BytesRead())
	}
}

func TestProgressReaderRateLimit(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 2048)
	stats := NewEngineStats("job", "rule")
	// 64 kbps is plenty for a small buffer in tests; just ensure it completes.
	pr := NewProgressReader(context.Background(), bytes.NewReader(data), stats, 1, int64(len(data)), 64)
	out, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(out) != len(data) {
		t.Fatalf("len=%d", len(out))
	}
}

func TestMatchPatterns(t *testing.T) {
	if !matchPatterns("photos/a.PNG", []string{".png"}, nil) {
		t.Fatal("expected include .png")
	}
	if matchPatterns("videos/a.mp4", nil, []string{".mp4"}) {
		t.Fatal("expected exclude .mp4")
	}
	if !matchPatterns("docs/report.pdf", []string{"docs/*"}, nil) {
		t.Fatal("expected docs/* match")
	}
}

func TestMapTargetKey(t *testing.T) {
	got := mapTargetKey("src/a/b.txt", "src/", "dst/")
	if got != "dst/a/b.txt" {
		t.Fatalf("got %q", got)
	}
}

func TestObjectsMatch(t *testing.T) {
	src := ObjectInfo{Key: "a", Size: 10, ETag: "abc"}
	dst := ObjectInfo{Key: "a", Size: 10, ETag: "abc"}
	if !objectsMatch(src, dst, "etag") {
		t.Fatal("etag match")
	}
	dst.ETag = "xyz"
	if objectsMatch(src, dst, "etag") {
		t.Fatal("etag mismatch should fail")
	}
	if !objectsMatch(src, dst, "size") {
		t.Fatal("size mode ignores etag")
	}
	dst.Size = 11
	if objectsMatch(src, dst, "size") {
		t.Fatal("size mismatch")
	}
}
