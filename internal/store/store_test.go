package store

import (
	"testing"
	"time"
	"webhook-dispatcher/internal/models"
)

func makeRecord(id string) models.EventRecord {
	return models.EventRecord{EventID: id, Payload: map[string]any{"x": 1}}
}

func TestSaveAndGet(t *testing.T) {
	s := New(time.Minute)
	s.Save(makeRecord("abc"))

	r, ok := s.Get("abc")
	if !ok || r.EventID != "abc" {
		t.Fatal("expected to retrieve saved record")
	}
}

func TestGetMissing(t *testing.T) {
	s := New(time.Minute)
	_, ok := s.Get("nope")
	if ok {
		t.Fatal("expected miss for unknown id")
	}
}

func TestTTLEviction(t *testing.T) {
	s := New(50 * time.Millisecond)
	s.Save(makeRecord("abc"))
	time.Sleep(60 * time.Millisecond)

	_, ok := s.Get("abc")
	if ok {
		t.Fatal("expected record to be expired")
	}
}

func TestAll_ExcludesExpired(t *testing.T) {
	s := New(50 * time.Millisecond)
	s.Save(makeRecord("fresh"))
	s.Save(makeRecord("stale"))

	time.Sleep(60 * time.Millisecond)
	s.Save(makeRecord("fresh2"))

	all := s.All()
	for _, r := range all {
		if r.EventID == "stale" || r.EventID == "fresh" {
			t.Fatalf("expected expired record %q to be excluded", r.EventID)
		}
	}
}


func TestEvictExpired(t *testing.T) {
	s := New(50 * time.Millisecond)
	s.Save(makeRecord("a"))
	s.Save(makeRecord("b"))
	time.Sleep(60 * time.Millisecond)

	n := s.EvictExpired()
	if n != 2 {
		t.Fatalf("expected 2 evicted, got %d", n)
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 remaining, got %d", s.Len())
	}
}