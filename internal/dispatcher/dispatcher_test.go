package dispatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"webhook-dispatcher/internal/models"
	"webhook-dispatcher/internal/signing"
	"webhook-dispatcher/internal/store"
)

func strPtr(s string) *string { return &s }

func makeRequest(url ...string) models.DispatchRequest {
	dests := make([]models.Destination, len(url))
	for i, u := range url {
		dests[i] = models.Destination{Type: models.DestinationGeneric, URL: u}
	}
	return models.DispatchRequest{
		Payload: map[string]any{"msg": "hello"},
		Destinations: dests,
		EventType: strPtr("test.event"),
	}
}

func TestBackoff_DoublesEachAttempt(t *testing.T) {
	base := 2 * time.Second
	if Backoff(0, base, time.Minute) != 2*time.Second { t.Fail() }
	if Backoff(1, base, time.Minute) != 4*time.Second { t.Fail() }
	if Backoff(2, base, time.Minute) != 8*time.Second { t.Fail() }
}

func TestBackoff_CappedAtMax(t *testing.T) {
	if Backoff(10, 2*time.Second, 60*time.Second) != 60*time.Second {
		t.Fatal("Expected cap at 60s")
	}
}

func TestDispatch_SuccessAllDestinations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := store.New(time.Minute)
	resp := Dispatch(context.Background(), makeRequest(srv.URL, srv.URL),
			"evt-001", srv.Client(), s, 3, time.Millisecond, time.Second, 5*time.Second)

	if resp.EventID != "evt-001" { t.Fatalf("Wrong event ID: %s", resp.EventID) }
	if len(resp.Results) != 2 { t.Fatalf("Expected 2 results, got %d", len(resp.Results)) }
	for _, r := range resp.Results {
		if r.Status != models.StatusSuccess { t.Fatalf("Expected success, got %s", r.Status) }
	}
}

func TestDispatch_RetriesOnFailuteThenSucceeds(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 2 { w.WriteHeader(200) } else { w.WriteHeader(500) }
	}))
	defer srv.Close()

	s := store.New(time.Minute)
	resp := Dispatch(context.Background(), makeRequest(srv.URL),
			"evt-001", srv.Client(), s, 3, time.Millisecond, time.Second, 5*time.Second)

	r := resp.Results[0]
	if r.Status != models.StatusSuccess { t.Fatal("Expected final success") }
	if len(r.Attempts) != 2 { t.Fatalf("Expected 2 attempts, got %d", len(r.Attempts)) }
	if r.Attempts[0].Status != models.StatusFailure { t.Fatal("Expected first attempt to fail") }
	if r.Attempts[1].Status != models.StatusSuccess { t.Fatal("Expected second to succeed") }
}

func TestDispatch_GivesUpAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	s := store.New(time.Minute)
	resp := Dispatch(context.Background(), makeRequest(srv.URL),
			"evt-003", srv.Client(), s, 3, time.Millisecond, time.Second, 5*time.Second)

	r := resp.Results[0]
	if r.Status != models.StatusFailure { t.Fatal("Expected final failure") }
	if len(r.Attempts) != 3 { t.Fatalf("Expected 3 attempts, got %d", len(r.Attempts)) }
}

func TestDispatch_FanOutIndependentPerDestination(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer bad.Close()

	s := store.New(time.Minute)
	resp := Dispatch(context.Background(), makeRequest(good.URL, bad.URL),
			"evt-004", http.DefaultClient, s, 2, time.Millisecond, time.Second, 5*time.Second)

	var goodR, badR models.DestinationResult
	for _, r := range resp.Results {
		if r.URL == good.URL { goodR = r }
		if r.URL == bad.URL  { badR = r }
	}
	if goodR.Status != models.StatusSuccess { t.Fatal("Expected good to succeed") }
	if badR.Status  != models.StatusFailure  { t.Fatal("Expected bad to fail") }	
}

func TestDispatch_SavesRecordToStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := store.New(time.Minute)
	Dispatch(context.Background(), makeRequest(srv.URL),
		"evt-005", srv.Client(), s, 1, time.Millisecond, time.Second, 5*time.Second)

	record, ok := s.Get("evt-005")
	if !ok { t.Fatal("Expected record to be saved") }
	if record.EventType == nil || *record.EventType != "test.event" {
		t.Fatal("Expected event type to be stored")
	}
}

func TestDispatch_SendsSignatureWhenSecretSet(t *testing.T) {
	var receivedSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	req := makeRequest(srv.URL)
	req.Secret = strPtr("mysecret")

	
	s := store.New(time.Minute)
	Dispatch(context.Background(), req,
		"evt-006", srv.Client(), s, 1, time.Millisecond, time.Second, 5*time.Second)

	if receivedSig == "" || receivedSig[:7] != "sha256=" {
		t.Fatalf("Expected X-Webhook-Signature, got %q", receivedSig)
	}
}

func TestDispatch_PerDestinationSecretOverridesGlobal(t *testing.T) {
	var receivedSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	destSecret := "dest-secret"
	req := models.DispatchRequest{
		Payload: map[string]any{"x": 1},
		Destinations: []models.Destination{
			{Type: models.DestinationGeneric, URL: srv.URL, Secret: &destSecret},
		},
		Secret:    strPtr("global-secret"),
		EventType: strPtr("test"),
	}

	s := store.New(time.Minute)
	Dispatch(context.Background(), req,
		"evt-007", srv.Client(), s, 1, time.Millisecond, time.Second, 5*time.Second)

	// Rebuild what the signature should be when signed with dest-secret
	body, _ := json.Marshal(map[string]any{"x": 1, "event_type": "test"})
	expected := signing.Sign(body, "dest-secret")
	if receivedSig != expected {
		t.Fatalf("Expected signature from dest-secret, got %q", receivedSig)
	}
}