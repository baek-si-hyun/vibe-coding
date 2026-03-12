package ops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SyncCheckpoint struct {
	Key             string            `json:"key"`
	LastAttemptAt   string            `json:"last_attempt_at,omitempty"`
	LastSuccessAt   string            `json:"last_success_at,omitempty"`
	LastTradingDate string            `json:"last_trading_date,omitempty"`
	LastAsOf        string            `json:"last_as_of,omitempty"`
	WindowStart     string            `json:"window_start,omitempty"`
	WindowEnd       string            `json:"window_end,omitempty"`
	Status          string            `json:"status,omitempty"`
	Note            string            `json:"note,omitempty"`
	Extra           map[string]string `json:"extra,omitempty"`
}

type SyncState struct {
	UpdatedAt   string                    `json:"updated_at"`
	Checkpoints map[string]SyncCheckpoint `json:"checkpoints"`
}

type StateStore struct {
	path string
	mu   sync.Mutex
}

func NewStateStore(path string) *StateStore {
	return &StateStore{
		path: strings.TrimSpace(path),
	}
}

func (s *StateStore) Path() string {
	return s.path
}

func (s *StateStore) Load() (SyncState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadUnlocked()
}

func (s *StateStore) Save(state SyncState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveUnlocked(normalizeState(state))
}

func (s *StateStore) GetCheckpoint(key string) (SyncCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadUnlocked()
	if err != nil {
		return SyncCheckpoint{}, err
	}
	checkpoint, ok := state.Checkpoints[strings.TrimSpace(key)]
	if !ok {
		return SyncCheckpoint{}, nil
	}
	return checkpoint, nil
}

func (s *StateStore) UpdateCheckpoint(key string, update func(*SyncCheckpoint)) (SyncState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadUnlocked()
	if err != nil {
		return SyncState{}, err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return SyncState{}, errors.New("checkpoint key is required")
	}

	checkpoint := state.Checkpoints[normalizedKey]
	checkpoint.Key = normalizedKey
	if checkpoint.Extra == nil {
		checkpoint.Extra = map[string]string{}
	}
	if update != nil {
		update(&checkpoint)
	}
	state.Checkpoints[normalizedKey] = checkpoint
	state.UpdatedAt = time.Now().Format(time.RFC3339)

	state = normalizeState(state)
	if err := s.saveUnlocked(state); err != nil {
		return SyncState{}, err
	}
	return state, nil
}

func (s *StateStore) loadUnlocked() (SyncState, error) {
	state := normalizeState(SyncState{})
	if s.path == "" {
		return state, nil
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return SyncState{}, err
	}
	if len(content) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return SyncState{}, err
	}
	return normalizeState(state), nil
}

func (s *StateStore) saveUnlocked(state SyncState) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(s.path, payload, 0o644)
}

func normalizeState(state SyncState) SyncState {
	if strings.TrimSpace(state.UpdatedAt) == "" {
		state.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	if state.Checkpoints == nil {
		state.Checkpoints = map[string]SyncCheckpoint{}
	}
	for key, checkpoint := range state.Checkpoints {
		checkpoint.Key = strings.TrimSpace(firstNonEmpty(checkpoint.Key, key))
		if checkpoint.Extra == nil {
			checkpoint.Extra = map[string]string{}
		}
		state.Checkpoints[key] = checkpoint
	}
	return state
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
