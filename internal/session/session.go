package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

const saveDebounce = 300 * time.Millisecond

type session struct {
	id       string
	messages []core.Message
	metadata map[string]string
	mu       sync.RWMutex
	store    *sessionStore

	// Saving a complete JSON session on each appended token/message makes the
	// request path depend on disk latency. Dirty generations and one debounced
	// timer coalesce those writes while Flush still offers a synchronous save at
	// process shutdown.
	saveMu    sync.Mutex
	saveTimer *time.Timer
	dirty     uint64
	saved     uint64
	deleted   bool
	persistMu sync.Mutex
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]core.Session
	maxMsg   int
	savePath string
	autoSave bool
}

func NewStore(maxMessages int, savePath string, autoSave bool) core.SessionStore {
	if maxMessages <= 0 {
		maxMessages = 100
	}
	s := &sessionStore{
		sessions: make(map[string]core.Session),
		maxMsg:   maxMessages,
		savePath: savePath,
		autoSave: autoSave,
	}
	_ = os.MkdirAll(savePath, 0755)
	s.loadSaved()
	return s
}

func (s *sessionStore) Create() core.Session {
	return s.CreateWithID(generateID())
}

func (s *sessionStore) CreateWithID(id string) core.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.sessions[id]; ok {
		return existing
	}

	sess := &session{
		id:       id,
		messages: make([]core.Message, 0, s.maxMsg),
		metadata: make(map[string]string),
		store:    s,
	}
	s.sessions[id] = sess
	return sess
}

func (s *sessionStore) Get(id string) (core.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *sessionStore) GetOrCreate(id string) core.Session {
	if sess, ok := s.Get(id); ok {
		return sess
	}
	return s.CreateWithID(id)
}

func (s *sessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id].(*session); ok {
		sess.cancelPendingSave()
	}
	delete(s.sessions, id)
	path := filepath.Join(s.savePath, id+".json")
	_ = os.Remove(path)
}

func (s *sessionStore) List() []core.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		list = append(list, sess)
	}
	return list
}

func (s *sessionStore) loadSaved() {
	files, err := os.ReadDir(s.savePath)
	if err != nil {
		return
	}
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.savePath, f.Name()))
		if err != nil {
			continue
		}
		var saved struct {
			ID       string            `json:"id"`
			Messages []core.Message    `json:"messages"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(data, &saved); err != nil || saved.ID == "" {
			continue
		}
		messages := trimMessages(saved.Messages, s.maxMsg)
		metadata := saved.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		s.sessions[saved.ID] = &session{
			id:       saved.ID,
			messages: messages,
			metadata: metadata,
			store:    s,
		}
	}
	logger.L().Debug("loaded saved sessions", "count", len(s.sessions))
}

func (s *session) ID() string {
	return s.id
}

func (s *session) Messages() []core.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a defensive copy to prevent callers from mutating the internal slice.
	msgs := make([]core.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

// MessagesSlice returns the last n messages without copying. Callers must
// not mutate the returned slice. Use only in performance-critical paths where
// the session is not concurrently modified while the result is consumed.
func (s *session) MessagesSlice(n int) []core.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.messages
	if n > 0 && len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	return msgs
}

func (s *session) Append(msg core.Message) {
	s.mu.Lock()
	s.messages = append(s.messages, msg)
	if s.store != nil {
		s.messages = trimMessages(s.messages, s.store.maxMsg)
	}
	s.mu.Unlock()
	s.markDirty()
}

func (s *session) Replace(msgs []core.Message) {
	s.mu.Lock()
	if s.store != nil {
		s.messages = trimMessages(msgs, s.store.maxMsg)
	} else {
		s.messages = append([]core.Message(nil), msgs...)
	}
	s.mu.Unlock()
	s.markDirty()
}

func (s *session) Clear() {
	s.mu.Lock()
	s.messages = make([]core.Message, 0, 64)
	s.mu.Unlock()
	s.markDirty()
}

func (s *session) SetMetadata(key, value string) {
	s.mu.Lock()
	s.metadata[key] = value
	s.mu.Unlock()
	s.markDirty()
}

func (s *session) GetMetadata(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.metadata[key]
	return v, ok
}

func (s *session) markDirty() {
	if s.store == nil || !s.store.autoSave {
		return
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if s.deleted {
		return
	}
	s.dirty++
	if s.saveTimer == nil {
		s.saveTimer = time.AfterFunc(saveDebounce, s.persistScheduled)
		return
	}
	s.saveTimer.Reset(saveDebounce)
}

func (s *session) persistScheduled() {
	s.saveMu.Lock()
	s.saveTimer = nil
	if s.deleted || s.saved >= s.dirty {
		s.saveMu.Unlock()
		return
	}
	target := s.dirty
	s.saveMu.Unlock()

	s.persist(target)
}

func (s *session) persist(target uint64) {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()

	s.saveMu.Lock()
	if s.deleted {
		s.saveMu.Unlock()
		return
	}
	s.saveMu.Unlock()

	s.mu.RLock()
	messages := append([]core.Message(nil), s.messages...)
	metadata := make(map[string]string, len(s.metadata))
	for k, v := range s.metadata {
		metadata[k] = v
	}
	store := s.store
	id := s.id
	s.mu.RUnlock()

	if store != nil {
		saveSessionTo(store, id, messages, metadata)
	}

	s.saveMu.Lock()
	if target > s.saved {
		s.saved = target
	}
	if !s.deleted && s.dirty > s.saved && s.saveTimer == nil {
		s.saveTimer = time.AfterFunc(saveDebounce, s.persistScheduled)
	}
	s.saveMu.Unlock()
}

// Flush synchronously persists pending updates. It is intentionally not part
// of core.Session, allowing existing third-party Session implementations to
// remain source-compatible. Call FlushSession during graceful shutdown.
func (s *session) Flush() {
	if s.store == nil || !s.store.autoSave {
		return
	}

	for {
		s.saveMu.Lock()
		if s.deleted || s.saved >= s.dirty {
			s.saveMu.Unlock()
			return
		}
		if s.saveTimer != nil {
			s.saveTimer.Stop()
			s.saveTimer = nil
		}
		target := s.dirty
		s.saveMu.Unlock()

		s.persist(target)
	}
}

func (s *session) cancelPendingSave() {
	s.saveMu.Lock()
	s.deleted = true
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveMu.Unlock()
}

// FlushSession persists a built-in session immediately when the caller is
// about to exit. Custom core.Session implementations are intentionally a no-op.
func FlushSession(sess core.Session) {
	if s, ok := sess.(*session); ok {
		s.Flush()
	}
}

func saveSessionTo(store *sessionStore, id string, messages []core.Message, metadata map[string]string) {
	data, err := json.Marshal(struct {
		ID       string            `json:"id"`
		Messages []core.Message    `json:"messages"`
		Metadata map[string]string `json:"metadata"`
	}{
		ID:       id,
		Messages: messages,
		Metadata: metadata,
	})
	if err != nil {
		logger.L().Warn("failed to marshal session", "id", id, "error", err)
		return
	}

	// Keep the session registered while writing so Delete cannot remove the
	// file and have an older timer recreate it immediately afterward.
	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.sessions[id]; !ok {
		return
	}
	path := filepath.Join(store.savePath, id+".json")
	tmp, err := os.CreateTemp(store.savePath, id+"-*.tmp")
	if err != nil {
		logger.L().Warn("failed to create session temp file", "id", id, "path", path, "error", err)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err == nil {
		err = tmp.Chmod(0600)
	}
	if err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err == nil {
		err = os.Rename(tmpName, path)
	}
	if err != nil {
		logger.L().Warn("failed to save session", "id", id, "path", path, "error", err)
	}
}

func trimMessages(messages []core.Message, max int) []core.Message {
	if max <= 0 || len(messages) <= max {
		return messages
	}
	trimmed := make([]core.Message, max)
	copy(trimmed, messages[len(messages)-max:])
	return trimmed
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
