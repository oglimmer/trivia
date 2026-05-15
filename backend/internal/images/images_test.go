package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// fixturePNG returns a w×h PNG with a simple gradient, so the encoder has real
// content to compress (a flat color would compress to the same bytes regardless
// of dimensions and wouldn't exercise resize meaningfully).
func fixturePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func TestProcessReencodesAndResizes(t *testing.T) {
	raw := fixturePNG(t, 800, 600)
	p, err := process(raw)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if p.width != 800 || p.height != 600 {
		t.Errorf("original dims: want 800x600, got %dx%d", p.width, p.height)
	}
	// JPEG magic: FF D8 FF
	if len(p.canonical) < 3 || p.canonical[0] != 0xff || p.canonical[1] != 0xd8 || p.canonical[2] != 0xff {
		t.Errorf("canonical bytes don't look like JPEG")
	}
	if len(p.sha) != 32 {
		t.Errorf("sha256 length: want 32, got %d", len(p.sha))
	}
	// Longest edge fits in the cap; aspect ratio preserved (within 1px rounding).
	if p.thumb.width > ThumbMaxEdge || p.thumb.height > ThumbMaxEdge {
		t.Errorf("thumb exceeds cap: %dx%d", p.thumb.width, p.thumb.height)
	}
	if p.thumb.width != ThumbMaxEdge {
		// 800x600 → longest edge 128 → thumb width should be 128.
		t.Errorf("thumb longest edge should hit cap: got %dx%d", p.thumb.width, p.thumb.height)
	}
	if p.medium.width > MediumMaxEdge || p.medium.height > MediumMaxEdge {
		t.Errorf("medium exceeds cap: %dx%d", p.medium.width, p.medium.height)
	}
	if p.medium.width != MediumMaxEdge {
		t.Errorf("medium longest edge should hit cap: got %dx%d", p.medium.width, p.medium.height)
	}
}

func TestProcessIsDeterministic(t *testing.T) {
	raw := fixturePNG(t, 400, 300)
	p1, err := process(raw)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := process(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p1.sha, p2.sha) {
		t.Errorf("sha differs between runs — dedupe will break")
	}
}

func TestProcessRejectsNonImage(t *testing.T) {
	if _, err := process([]byte("not an image, certainly not SVG")); err == nil {
		t.Fatal("expected decode failure")
	}
}

func TestETagShapes(t *testing.T) {
	sha := make([]byte, 32)
	for i := range sha {
		sha[i] = byte(i)
	}
	orig := etag(sha, "")
	if !strings.HasPrefix(orig, `"`) || !strings.HasSuffix(orig, `"`) {
		t.Errorf("etag missing quotes: %q", orig)
	}
	if len(orig) != 1+64+1 {
		t.Errorf("original etag length: want 66, got %d (%q)", len(orig), orig)
	}
	thumb := etag(sha, "thumb")
	if !strings.Contains(thumb, "-thumb") {
		t.Errorf("variant etag missing kind suffix: %q", thumb)
	}
	if orig == thumb {
		t.Errorf("variant etag must differ from original")
	}
}
