package urlcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	URL       string          `json:"url"`
	Body      json.RawMessage `json:"body"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Store struct {
	dir       string
	mu        sync.RWMutex
	memory    map[string]Entry
	active    map[string]struct{}
	capture   map[string]struct{}
	capturing bool
}

func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("directorio de cache vacío")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("crear cache: %w", err)
	}
	return &Store{
		dir:    dir,
		memory: make(map[string]Entry),
		active: make(map[string]struct{}),
	}, nil
}

func (s *Store) Get(rawURL string) ([]byte, bool, error) {
	s.Track(rawURL)

	s.mu.RLock()
	entry, ok := s.memory[rawURL]
	s.mu.RUnlock()
	if ok {
		return bytes.Clone(entry.Body), true, nil
	}

	data, err := os.ReadFile(s.path(rawURL))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("leer cache: %w", err)
	}
	if err := json.Unmarshal(data, &entry); err != nil || entry.URL != rawURL || !json.Valid(entry.Body) {
		_ = os.Remove(s.path(rawURL))
		return nil, false, nil
	}

	s.mu.Lock()
	s.memory[rawURL] = entry
	s.mu.Unlock()
	return bytes.Clone(entry.Body), true, nil
}

func (s *Store) Put(rawURL string, body []byte) (bool, error) {
	if !json.Valid(body) {
		return false, errors.New("respuesta no es JSON válido")
	}

	s.mu.RLock()
	current, ok := s.memory[rawURL]
	s.mu.RUnlock()
	if !ok {
		if _, hit, err := s.Get(rawURL); err == nil && hit {
			s.mu.RLock()
			current, ok = s.memory[rawURL]
			s.mu.RUnlock()
		}
	}
	changed := !ok || !bytes.Equal(current.Body, body)
	entry := Entry{URL: rawURL, Body: bytes.Clone(body), UpdatedAt: time.Now().UTC()}
	if !changed {
		entry.Body = current.Body
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return false, fmt.Errorf("codificar cache: %w", err)
	}
	if err := s.writeAtomic(rawURL, encoded); err != nil {
		return false, err
	}
	s.mu.Lock()
	s.memory[rawURL] = entry
	s.mu.Unlock()
	return changed, nil
}

func (s *Store) Delete(rawURL string) {
	s.mu.Lock()
	delete(s.memory, rawURL)
	s.mu.Unlock()
	_ = os.Remove(s.path(rawURL))
}

func (s *Store) Track(rawURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.capturing {
		s.capture[rawURL] = struct{}{}
		return
	}
	s.active[rawURL] = struct{}{}
}

func (s *Store) BeginCapture() {
	s.mu.Lock()
	s.capture = make(map[string]struct{})
	s.capturing = true
	s.mu.Unlock()
}

func (s *Store) CommitCapture() {
	s.mu.Lock()
	if s.capturing {
		s.active = s.capture
	}
	s.capture = nil
	s.capturing = false
	s.mu.Unlock()
}

func (s *Store) AbortCapture() {
	s.mu.Lock()
	s.capture = nil
	s.capturing = false
	s.mu.Unlock()
}

func (s *Store) ActiveURLs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	urls := make([]string, 0, len(s.active))
	for rawURL := range s.active {
		urls = append(urls, rawURL)
	}
	return urls
}

func (s *Store) path(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *Store) writeAtomic(rawURL string, data []byte) error {
	temporary, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("crear cache temporal: %w", err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("proteger cache: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("escribir cache: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sincronizar cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("cerrar cache: %w", err)
	}
	if err := os.Rename(name, s.path(rawURL)); err != nil {
		return fmt.Errorf("publicar cache: %w", err)
	}
	return nil
}
