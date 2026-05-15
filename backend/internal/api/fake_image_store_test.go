package api

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/oglimmer/trivia/backend/internal/images"
)

// fakeImageStore is the in-memory ImageStore used by handler tests so they can
// pre-seed valid photoImageIds without standing up Postgres + the real
// upload pipeline.
type fakeImageStore struct {
	mu                 sync.Mutex
	ids                map[string]bool
	deleteOrphansCalls []time.Time
}

func newFakeImageStore(seeded ...string) *fakeImageStore {
	f := &fakeImageStore{ids: map[string]bool{}}
	for _, id := range seeded {
		f.ids[id] = true
	}
	return f
}

func (f *fakeImageStore) Store(_ context.Context, _ io.Reader) (string, error) {
	// Handlers under test do not call Store; image creation happens through the
	// dedicated multipart endpoint, which has its own coverage.
	return "", nil
}

func (f *fakeImageStore) Get(_ context.Context, id string) (*images.Blob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ids[id] {
		return nil, images.ErrNotFound
	}
	return &images.Blob{Mime: "image/jpeg"}, nil
}

func (f *fakeImageStore) GetVariant(_ context.Context, id, _ string) (*images.Blob, error) {
	return f.Get(context.Background(), id)
}

// deleteOrphansCalls records each grace cutoff DeleteOrphans was invoked with,
// so tests can assert the deleteGame handler fired the sweep.
func (f *fakeImageStore) DeleteOrphans(_ context.Context, olderThan time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteOrphansCalls = append(f.deleteOrphansCalls, olderThan)
	return 0, nil
}
