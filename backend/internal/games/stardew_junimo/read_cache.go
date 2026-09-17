package stardew_junimo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type readCacheEntry[T any] struct {
	key     string
	value   T
	err     error
	expires time.Time
	done    chan struct{}
}

// At most one reader per instance runs at a time. Waiters can cancel without
// canceling another browser's read; the shared work has its own bounded context.
type instanceReadCache[T any] struct {
	mu      sync.Mutex
	entries map[string]*readCacheEntry[T]
}

func (c *instanceReadCache[T]) get(ctx context.Context, instanceID, key string, ttl time.Duration, read func(context.Context) (T, error)) (T, error) {
	for {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		c.mu.Lock()
		if c.entries == nil {
			c.entries = make(map[string]*readCacheEntry[T])
		}
		entry := c.entries[instanceID]
		if entry != nil && entry.done == nil && entry.key == key && time.Now().Before(entry.expires) {
			c.mu.Unlock()
			return entry.value, entry.err
		}
		if entry == nil || entry.done == nil {
			// Bound abandoned-instance retention without evicting active work.
			if len(c.entries) >= 128 {
				for id, old := range c.entries {
					if old.done == nil {
						delete(c.entries, id)
					}
				}
			}
			entry = &readCacheEntry[T]{key: key, done: make(chan struct{})}
			c.entries[instanceID] = entry
			go func(entry *readCacheEntry[T]) {
				readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
				defer cancel()
				value, err := read(readCtx)
				c.mu.Lock()
				defer c.mu.Unlock()
				entry.value, entry.err, entry.expires = value, err, time.Now().Add(ttl)
				if err != nil {
					entry.expires = time.Time{}
				}
				close(entry.done)
				entry.done = nil
			}(entry)
		}
		done := entry.done
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-done:
			// Failed reads are shared by current waiters but retried by the next call.
			c.mu.Lock()
			value, err, same := entry.value, entry.err, entry.key == key
			if err != nil {
				entry.expires = time.Time{}
			}
			c.mu.Unlock()
			if same {
				return value, err
			}
		}
	}
}

func fileReadKey(paths ...string) string {
	var key strings.Builder
	for _, path := range paths {
		fmt.Fprint(&key, path, "\x00")
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(&key, "%d:%d;", info.Size(), info.ModTime().UnixNano())
		} else {
			fmt.Fprint(&key, "missing;")
		}
	}
	return key.String()
}

type RuntimeDisplayInspection struct {
	RuntimeComponentsInspection
	SMAPI *SMAPIUpdateInfo `json:"smapi,omitempty"`
}

// Display only. Mutation preflights intentionally call the uncached inspectors.
func (d *Driver) InspectRuntimeDisplay(ctx context.Context, instance registry.Instance) (RuntimeDisplayInspection, error) {
	key := instance.State + fileReadKey(filepath.Join(instance.DataDir, ".env"), filepath.Join(instance.DataDir, "docker-compose.yml"))
	return d.runtimeDisplayReads.get(ctx, instance.ID, key, 30*time.Second, func(ctx context.Context) (RuntimeDisplayInspection, error) {
		components, err := d.InspectRuntimeComponents(ctx, instance)
		if err != nil {
			return RuntimeDisplayInspection{}, err
		}
		smapi, err := d.inspectSMAPIUpdate(ctx, instance, &components)
		return RuntimeDisplayInspection{RuntimeComponentsInspection: components, SMAPI: &smapi}, err
	})
}
