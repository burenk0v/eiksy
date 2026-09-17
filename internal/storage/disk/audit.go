package disk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"eiksy/internal/domain/audit"
)

const maxAuditEvents = 5000

type AuditRepository struct {
	mu   sync.Mutex
	path string
}

func NewAuditRepository(baseDir string) *AuditRepository {
	return &AuditRepository{path: filepath.Join(baseDir, "audit.jsonl")}
}

func (r *AuditRepository) Append(ctx context.Context, event audit.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure audit log: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	data = append(data, '\n')
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync audit log: %w", err)
	}
	return nil
}

func (r *AuditRepository) List(ctx context.Context, limit int) ([]audit.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxAuditEvents {
		limit = maxAuditEvents
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return []audit.Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	events := make([]audit.Event, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event audit.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode audit event: %w", err)
		}
		events = append(events, event)
		if len(events) > limit {
			events = events[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read audit log: %w", err)
	}
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	return events, nil
}
