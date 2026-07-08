package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

type session struct {
	id       string
	messages []core.Message
	metadata map[string]string
	mu       sync.RWMutex
	store    *sessionStore
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]core.Session
	maxMsg   int
	savePath string
	autoSave bool
}

func NewStore(maxMessages int, savePath string, autoSave bool) core.SessionStore {
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
		msgs := sess.Messages()
		if len(msgs) > s.maxMsg {
			trimmed := make([]core.Message, s.maxMsg)
			copy(trimmed, msgs[len(msgs)-s.maxMsg:])
			sess.Replace(trimmed)
		}
		return sess
	}
	return s.CreateWithID(id)
}

func (s *sessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
		var sess session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		sess.store = s
		s.sessions[sess.id] = &sess
	}
	logger.L().Debug("loaded saved sessions", "count", len(s.sessions))
}

func (s *session) ID() string {
	return s.id
}

func (s *session) Messages() []core.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := make([]core.Message, len(s.messages))
	copy(msgs, s.messages)
	return msgs
}

func (s *session) Append(msg core.Message) {
	s.mu.Lock()
	s.messages = append(s.messages, msg)
	s.mu.Unlock()
	if s.store != nil && s.store.autoSave {
		saveSessionTo(s.store, s.id, s.messages, s.metadata)
	}
}

func saveSessionTo(store *sessionStore, id string, messages []core.Message, metadata map[string]string) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	data, err := json.Marshal(map[string]any{
		"id":       id,
		"messages": messages,
		"metadata": metadata,
	})
	if err != nil {
		logger.L().Warn("failed to marshal session", "id", id, "error", err)
		return
	}
	path := filepath.Join(store.savePath, id+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		logger.L().Warn("failed to save session", "id", id, "path", path, "error", err)
	}
}

func (s *session) Replace(msgs []core.Message) {
	s.mu.Lock()
	s.messages = msgs
	s.mu.Unlock()
	if s.store != nil && s.store.autoSave {
		saveSessionTo(s.store, s.id, s.messages, s.metadata)
	}
}

func (s *session) Clear() {
	s.mu.Lock()
	s.messages = make([]core.Message, 0, 64)
	s.mu.Unlock()
}

func (s *session) SetMetadata(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
}

func (s *session) GetMetadata(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.metadata[key]
	return v, ok
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
