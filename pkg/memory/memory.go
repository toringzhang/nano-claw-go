package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Memory interface {
	Run(ctx context.Context)
	Append(msg openai.ChatCompletionMessage)
	Load() error
	History() []history
	Length() int
	LastHistory() history
	HistoryMessages() []openai.ChatCompletionMessage
}

type history struct {
	openai.ChatCompletionMessage
	Timestamp string `json:"timestamp"`
}

type memory struct {
	mutex sync.RWMutex
	key   string
	// history stores all chat messages with timestamps
	history []history
	length  int
	hasSync bool
}

// NewMemory creates a new Memory instance, auto-creates memory/ directory
func NewMemory(key string) Memory {
	if err := os.MkdirAll(filepath.Join("memory"), 0755); err != nil {
		panic(err)
	}
	return &memory{
		key:     key,
		history: []history{},
	}
}

// Run periodically saves history to disk (every 10 seconds)
func (m *memory) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.saveHistory()
		}
	}
}

func (m *memory) Append(msg openai.ChatCompletionMessage) {
	item := history{msg, time.Now().Format(time.RFC3339)}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.hasSync = false
	m.history = append(m.history, item)
	m.length++
}

func (m *memory) Load() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.hasSync {
		return errors.New("history hasn't sync")
	}
	data, err := os.ReadFile(m.fileName())
	if err != nil {
		return err
	}
	var h []history
	err = json.Unmarshal(data, &h)
	if err != nil {
		return err
	}
	m.history = h
	return nil
}

func (m *memory) LastHistory() history {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if len(m.history) == 0 {
		return history{}
	}
	return m.history[len(m.history)-1]
}

func (m *memory) History() []history {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	output := make([]history, len(m.history))
	copy(output, m.history)
	return output
}

func (m *memory) HistoryMessages() []openai.ChatCompletionMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	messages := make([]openai.ChatCompletionMessage, len(m.history))
	for idx := range m.history {
		messages[idx] = m.history[idx].ChatCompletionMessage
	}
	return messages
}

func (m *memory) Length() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.length
}

// saveHistory uses atomic write (temp file + rename) to prevent corruption
func (m *memory) saveHistory() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.hasSync = true
	if len(m.history) == 0 {
		return
	}
	data, err := json.Marshal(m.history)
	if err != nil {
		fmt.Printf("history marshal error: %s\n", err)
		return
	}
	err = safeWriteFile(m.fileName(), data, 0644)
	if err != nil {
		fmt.Printf("history save error: %s\n", err)
	}
}

func (m *memory) fileName() string {
	return filepath.Join("memory", m.key+".json")
}

func safeWriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	tmpFile, err := os.CreateTemp(dir, "tmp-"+filepath.Base(filename)+"-*")
	if err != nil {
		return fmt.Errorf("create temp file error: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write data error: %w", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		tmpFile.Close()
		return fmt.Errorf("chmod error: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("fsync error: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file error: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), filename); err != nil {
		return fmt.Errorf("rename error: %w", err)
	}
	return nil
}
