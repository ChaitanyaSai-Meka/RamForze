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
				now := time.Now().UTC()
				entries[i].CompletedAt = &now
				entries[i].Result = result
			} else {
				entries[i].CompletedAt = nil
				entries[i].Result = nil
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

	for i := range entries {
		if entries[i].TaskID == taskID {
			return &entries[i], nil
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
	dir := filepath.Dir(j.filePath)

	tmp, err := os.CreateTemp(dir, journalFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("could not create temp journal: %w", err)
	}
	tmpPath := tmp.Name()
	keepTemp := false
	defer func() {
		_ = tmp.Close()
		if !keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	mode := os.FileMode(0644)
	info, err := os.Stat(j.filePath)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not stat journal file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("could not set temp journal permissions: %w", err)
	}

	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("could not serialize entry: %w", err)
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("could not write temp journal entry: %w", err)
		}
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("could not sync temp journal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close temp journal: %w", err)
	}

	if err := os.Rename(tmpPath, j.filePath); err != nil {
		return fmt.Errorf("could not replace journal atomically: %w", err)
	}

	keepTemp = true
	return nil
}
