package translate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Cache keeps translations in a JSON file, so that a dry run followed by
// the import, or a run interrupted halfway, does not translate twice.
type Cache struct {
	path    string
	mu      sync.Mutex
	entries map[string]string
	dirty   int
}

// OpenCache reads the cache file at path, if it exists.
func OpenCache(path string) (*Cache, error) {
	c := &Cache{path: path, entries: map[string]string{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &c.entries); err != nil {
		return nil, err
	}
	return c, nil
}

// Key identifies a translation: the same text translated with another
// model, direction or kind is another entry.
func Key(model, from, to, kind, text string) string {
	sum := sha256.Sum256([]byte(model + "\x00" + from + "\x00" + to + "\x00" + kind + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

// Get returns the cached translation for key.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

// Put stores a translation, and writes the file every 20 new entries.
func (c *Cache) Put(key, value string) error {
	c.mu.Lock()
	c.entries[key] = value
	c.dirty++
	flush := c.dirty >= 20
	c.mu.Unlock()
	if flush {
		return c.Save()
	}
	return nil
}

// Save writes the file, through a temporary file so that an interrupted
// write leaves the previous one.
func (c *Cache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := json.Marshal(c.entries)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".vigie-translation-cache-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), c.path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	c.dirty = 0
	return nil
}
