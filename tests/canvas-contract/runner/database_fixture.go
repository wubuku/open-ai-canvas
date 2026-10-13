package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func SetCanvasUpdatedAt(ctx context.Context, config Config, projectIDs []string, updatedAt time.Time) error {
	if len(projectIDs) == 0 || len(projectIDs) > 32 || updatedAt.IsZero() {
		return fmt.Errorf("canvas timestamp fixture requires 1..32 distinct IDs and a timestamp")
	}
	seen := make(map[string]bool, len(projectIDs))
	for _, identity := range projectIDs {
		if identity == "" || identity != strings.TrimSpace(identity) || len(identity) > 80 || seen[identity] {
			return fmt.Errorf("canvas timestamp fixture requires distinct nonempty IDs of at most 80 bytes")
		}
		seen[identity] = true
	}
	timestamp := updatedAt.UTC().Format(time.RFC3339Nano)
	payload := map[string]any{"projectIds": projectIDs, "updatedAt": timestamp}
	return callDatabaseFixture(ctx, config, "/set-canvas-updated-at", payload, func(result map[string]any) error {
		updated, ok := result["updated"].(float64)
		if !ok || updated != float64(len(projectIDs)) || result["updatedAt"] != timestamp {
			return fmt.Errorf("database fixture did not confirm the exact canvas count and timestamp")
		}
		return nil
	})
}

func ExpireUploadReservation(ctx context.Context, config Config, uploadID string) error {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return fmt.Errorf("upload reservation fixture requires an upload ID")
	}
	return callDatabaseFixture(ctx, config, "/expire-upload-reservation", map[string]string{"uploadId": uploadID}, func(result map[string]any) error {
		if result["expired"] != true || result["uploadId"] != uploadID {
			return fmt.Errorf("database fixture did not confirm upload reservation %q", uploadID)
		}
		return nil
	})
}

func callDatabaseFixture(ctx context.Context, config Config, path string, payload any, validate func(map[string]any) error) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.FixtureURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+config.FixtureToken)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: config.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("database fixture %s returned HTTP %d", path, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil {
		return fmt.Errorf("database fixture response read failed")
	}
	var result map[string]any
	if len(body) > 16<<10 || json.Unmarshal(body, &result) != nil {
		return fmt.Errorf("database fixture returned an invalid or oversized JSON response")
	}
	return validate(result)
}
