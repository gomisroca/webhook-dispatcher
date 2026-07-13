package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
	"webhook-dispatcher/internal/formatters"
	"webhook-dispatcher/internal/models"
	"webhook-dispatcher/internal/signing"
	"webhook-dispatcher/internal/store"
)

// Compute delay before attempt
func Backoff(n int, base, max time.Duration) time.Duration {
	delay := time.Duration(float64(base) * math.Pow(2, float64(n)))
	if delay > max {
		return max
	}
	return delay
}

func ptr[T any](v T) *T { return &v }


// Makes one HTTP POST attempt. Returns (httpStatus, errorMsg)
func deliverOnce(
	ctx context.Context,
	client *http.Client,
	url string,
	body map[string]any,
	secret *string,
	timeout time.Duration,
) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("Marshaling body: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return 0, fmt.Errorf("Building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != nil && *secret != "" {
		req.Header.Set("X-Webhook-Signature", signing.Sign(raw, *secret))
	}

	
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Non-2xx response: %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// Runs the retry loop for a single destination. Runs inside its own goroutine (launched by Dispatch fn)
func deliverWithRetries(
	ctx context.Context,
	client *http.Client,
	dest models.Destination,
	body map[string]any,
	globalSecret *string,
	maxAttempts int,
	backoffBase, backoffMax, deliveryTimeout time.Duration,
) models.DestinationResult {
	url := dest.URL
	effectiveSecret := dest.Secret
	if effectiveSecret == nil {
		effectiveSecret = globalSecret
	}

	var attempts []models.DeliveryAttempt

	for n := 1; n <= maxAttempts; n++ {
		httpStatus, err := deliverOnce(ctx, client, url, body, effectiveSecret, deliveryTimeout)

		attempt := models.DeliveryAttempt{
			AttemptNumber: n,
			Timestamp:     time.Now().UTC(),
		}
		if err == nil {
			attempt.Status = models.StatusSuccess
			attempt.HTTPStatus = ptr(httpStatus)
		} else {
			attempt.Status = models.StatusFailure
			if httpStatus != 0 {
				attempt.HTTPStatus = ptr(httpStatus)
			}
			attempt.Error = ptr(err.Error())
			log.Printf("Delivery attempt %d to %s failed: %v", n, url, err)
		}
		attempts = append(attempts, attempt)

		if err == nil {
			log.Printf("Delivered to %s on attempt %d", url, n)
			break
		}

		if n < maxAttempts {
			delay := Backoff(n-1, backoffBase, backoffMax)
			log.Printf("Retrying %s in %s", url, delay)
			time.Sleep(delay)
		}
	}

	finalStatus := attempts[len(attempts)-1].Status
	return models.DestinationResult{
		DestinationType: dest.Type,
		URL:             url,
		Status:          finalStatus,
		Attempts:        attempts,
	}
}

// Fans out delivery attempts to all destinations, using one goroutine per destination
func Dispatch(
	ctx context.Context,
	req models.DispatchRequest,
	eventID string,
	client *http.Client,
	s *store.EventStore,
	maxAttempts int,
	backoffBase, backoffMax, deliveryTimeout time.Duration,
) models.DispatchResponse {
	results := make([]models.DestinationResult, len(req.Destinations))
	var wg sync.WaitGroup

	for i, dest := range req.Destinations {
		wg.Add(1)
		go func(i int, dest models.Destination) {
			defer wg.Done()
			body := formatters.Format(dest.Type, req.Payload, req.EventType)
			results[i] = deliverWithRetries(
				ctx, client, dest, body, req.Secret,
				maxAttempts, backoffBase, backoffMax, deliveryTimeout,
			)
		}(i, dest)
	}

	wg.Wait()

	s.Save(models.EventRecord{
		EventID: eventID,
		EventType: req.EventType,
		Payload: req.Payload,
		CreatedAt: time.Now().UTC(),
		Results: results,
	})

	return models.DispatchResponse{EventID: eventID, Results: results}
}