// In-memory store for webhooks
package store

import (
	"sync"
	"time"
	"webhook-dispatcher/internal/models"
)

type entry struct {
	record 		models.EventRecord
	expiresAt 	time.Time
}

type EventStore struct {
	mu 		sync.RWMutex
	records map[string]entry
	ttl 	time.Duration
}

func New(ttl time.Duration) *EventStore {
	return &EventStore{
		records: make(map[string]entry),
		ttl: ttl,
	}
}

func (s *EventStore) Save(record models.EventRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.EventID] = entry{
		record:    record,
		expiresAt: time.Now().Add(s.ttl),
	}
}

func (s *EventStore) Get(id string) (models.EventRecord, bool) {
	s.mu.RLock()
	e, ok := s.records[id]
	s.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		return models.EventRecord{}, false
	}
	return e.record, true
}

func (s *EventStore) All() []models.EventRecord {
	now := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.EventRecord, 0, len(s.records))
	for _, e := range s.records {
		if !now.After(e.expiresAt) { // Prune expired records from the list
			out = append(out, e.record)
		}
	}
	return out
}

func (s *EventStore) EvictExpired() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var evicted int
	for id, e := range s.records {
		if now.After(e.expiresAt) {
			delete(s.records, id)
			evicted++
		}
	}
	return evicted
}

func (s *EventStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
