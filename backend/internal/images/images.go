// Package images stores user-uploaded photos as content-addressed JPEG blobs
// with pre-rendered thumb/medium variants. See docs/image-architecture.md.
package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"io"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/disintegration/imaging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxUploadBytes  = 8 << 20 // 8 MiB
	OriginalQuality = 85
	VariantQuality  = 80
	ThumbMaxEdge    = 128
	MediumMaxEdge   = 640

	jpegMime = "image/jpeg"
)

var ErrNotFound = errors.New("image not found")
var ErrTooLarge = fmt.Errorf("upload exceeds %d bytes", MaxUploadBytes)

// Blob is what the serving layer hands to clients: bytes plus the metadata it
// needs for Content-Type / ETag.
type Blob struct {
	Mime   string
	Width  int
	Height int
	Bytes  []byte
	// ETag is a stable, content-derived value: hex sha256 of the served bytes
	// for the original, or "<sha>-<kind>" for variants.
	ETag string
}

// Service is the upload + retrieval entry point.
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Store reads the upload, re-encodes to JPEG (stripping EXIF), hashes it,
// dedupes on sha256, and on first insert writes the original plus a thumb and
// medium variant in one tx. Returns the image's UUID.
//
// Callers should still wrap the request body with http.MaxBytesReader so the
// transport rejects oversize uploads before any work happens here; the
// in-package cap is a defense in depth.
func (s *Service) Store(ctx context.Context, r io.Reader) (string, error) {
	limited := io.LimitReader(r, MaxUploadBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read upload: %w", err)
	}
	if len(raw) > MaxUploadBytes {
		return "", ErrTooLarge
	}

	p, err := process(raw)
	if err != nil {
		return "", err
	}

	// Fast path: row already exists.
	var existingID string
	err = s.pool.QueryRow(ctx, `SELECT id FROM images WHERE sha256=$1`, p.sha).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("dedupe lookup: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Two uploaders racing on the same sha both arrive here. ON CONFLICT lets
	// the loser observe the winner's id without a unique-violation error.
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO images (sha256, mime, width, height, bytes, byte_size)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (sha256) DO UPDATE SET sha256 = images.sha256
		RETURNING id
	`, p.sha, jpegMime, p.width, p.height, p.canonical, len(p.canonical)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert image: %w", err)
	}

	if err := insertVariant(ctx, tx, id, "thumb", p.thumb); err != nil {
		return "", err
	}
	if err := insertVariant(ctx, tx, id, "medium", p.medium); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func insertVariant(ctx context.Context, tx pgx.Tx, id, kind string, v variant) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO image_variants (image_id, kind, mime, width, height, bytes, byte_size)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (image_id, kind) DO NOTHING
	`, id, kind, jpegMime, v.width, v.height, v.bytes, len(v.bytes))
	if err != nil {
		return fmt.Errorf("insert %s variant: %w", kind, err)
	}
	return nil
}

// Get returns the canonical (original) blob.
func (s *Service) Get(ctx context.Context, id string) (*Blob, error) {
	b := &Blob{}
	var sha []byte
	err := s.pool.QueryRow(ctx, `
		SELECT mime, sha256, width, height, bytes FROM images WHERE id=$1
	`, id).Scan(&b.Mime, &sha, &b.Width, &b.Height, &b.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.ETag = etag(sha, "")
	return b, nil
}

// GetVariant returns a derived size ("thumb" or "medium").
func (s *Service) GetVariant(ctx context.Context, id, kind string) (*Blob, error) {
	b := &Blob{}
	var sha []byte
	err := s.pool.QueryRow(ctx, `
		SELECT v.mime, i.sha256, v.width, v.height, v.bytes
		FROM image_variants v
		JOIN images i ON i.id = v.image_id
		WHERE v.image_id=$1 AND v.kind=$2
	`, id, kind).Scan(&b.Mime, &sha, &b.Width, &b.Height, &b.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.ETag = etag(sha, kind)
	return b, nil
}

// ValidVariant reports whether kind is one of the supported variant names.
func ValidVariant(kind string) bool {
	return kind == "thumb" || kind == "medium"
}

// DeleteOrphans removes images that no user or question still points at and
// that were created before olderThan. The age filter avoids racing the
// upload-then-attach flow: a freshly uploaded image is briefly an "orphan"
// until the client completes the join/putQuestion call. Returns the number of
// rows deleted. image_variants are removed via ON DELETE CASCADE.
func (s *Service) DeleteOrphans(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM images i
		WHERE i.created_at < $1
		  AND NOT EXISTS (SELECT 1 FROM users     u WHERE u.photo_image_id = i.id)
		  AND NOT EXISTS (SELECT 1 FROM questions q WHERE q.photo_image_id = i.id)
	`, olderThan)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// processed is the output of the pure encoding pipeline: canonical original
// bytes, content hash, and both pre-rendered variants — everything Store needs
// before it touches Postgres.
type processed struct {
	canonical []byte
	sha       []byte
	width     int
	height    int
	thumb     variant
	medium    variant
}

type variant struct {
	bytes  []byte
	width  int
	height int
}

// process is the encode/resize/hash pipeline. Split out from Store so it's
// trivial to unit-test without a database.
func process(raw []byte) (*processed, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	canonical, err := encodeJPEG(img, OriginalQuality)
	if err != nil {
		return nil, fmt.Errorf("encode original: %w", err)
	}
	sum := sha256.Sum256(canonical)

	thumb, err := makeVariant(img, ThumbMaxEdge)
	if err != nil {
		return nil, fmt.Errorf("encode thumb: %w", err)
	}
	medium, err := makeVariant(img, MediumMaxEdge)
	if err != nil {
		return nil, fmt.Errorf("encode medium: %w", err)
	}

	b := img.Bounds()
	return &processed{
		canonical: canonical,
		sha:       sum[:],
		width:     b.Dx(),
		height:    b.Dy(),
		thumb:     thumb,
		medium:    medium,
	}, nil
}

func makeVariant(src image.Image, maxEdge int) (variant, error) {
	resized := imaging.Fit(src, maxEdge, maxEdge, imaging.Lanczos)
	bs, err := encodeJPEG(resized, VariantQuality)
	if err != nil {
		return variant{}, err
	}
	rb := resized.Bounds()
	return variant{bytes: bs, width: rb.Dx(), height: rb.Dy()}, nil
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func etag(sha []byte, kind string) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, len(sha)*2+len(kind)+3)
	out = append(out, '"')
	for _, b := range sha {
		out = append(out, hex[b>>4], hex[b&0x0f])
	}
	if kind != "" {
		out = append(out, '-')
		out = append(out, kind...)
	}
	out = append(out, '"')
	return string(out)
}
