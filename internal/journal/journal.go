package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chaitanyasai-meka/Ramforze/pkg/types"
)

const journalFileName = "journal.ndjson"

type Journal struct {
	mu       sync.Mutex
	filePath string
}

func New() (*Journal, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home directory: %w", err)
	}

	dir := filepath.Join(home, ".ramforze")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("could not create .ramforze directory: %w", err)
	}

	return &Journal{
		filePath: filepath.Join(dir, journalFileName),
	}, nil
}

func (j *Journal) Write(entry types.JournalEntry) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	f, err := os.OpenFile(j.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open journal: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("could not serialize journal entry: %w", err)
	}

	_, err = f.Write(append(line, '\n'))
	return err
}

func (j *Journal) UpdateStatus(taskID string, status string, result *types.TaskResult) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	entries, err := j.readAll()
	if err != nil {
		return err
	}

	updated := false
	for i, e := range entries {
		if e.TaskID == taskID {
			entries[i].Status = status
			if status == "completed" || status == "failed" || status == "timed_out" {
				now := time.Now()
				entries[i].CompletedAt = &now
				entries[i].Result = result
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("task %s not found in journal", taskID)
	}

	return j.writeAll(entries)
}

func (j *Journal) GetDispatched() ([]types.JournalEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	all, err := j.readAll()
	if err != nil {
		return nil, err
	}

	var dispatched []types.JournalEntry
	for _, e := range all {
		if e.Status == "dispatched" {
			dispatched = append(dispatched, e)
		}
	}
	return dispatched, nil
}

func (j *Journal) GetByTaskID(taskID string) (*types.JournalEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	entries, err := j.readAll()
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.TaskID == taskID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("task %s not found in journal", taskID)
}

func (j *Journal) readAll() ([]types.JournalEntry, error) {
	data, err := os.ReadFile(j.filePath)
	if errors.Is(err, os.ErrNotExist){
		return []types.JournalEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read journal: %w", err)
	}

	var entries []types.JournalEntry
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry types.JournalEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("corrupt journal entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (j *Journal) writeAll(entries []types.JournalEntry) error {
	f, err := os.OpenFile(j.filePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open journal for rewrite: %w", err)
	}
	defer f.Close()

	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("could not serialize entry: %w", err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("could not write entry: %w", err)
		}
	}
	return nil
}
