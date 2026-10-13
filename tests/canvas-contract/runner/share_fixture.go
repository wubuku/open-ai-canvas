package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func ExpireShare(ctx context.Context, config Config, projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("share expiry fixture requires a project ID")
	}
	payload, err := json.Marshal(map[string]string{"projectId": projectID})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.FixtureURL+"/expire-share", bytes.NewReader(payload))
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
		return fmt.Errorf("share expiry fixture returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Expired   bool   `json:"expired"`
		ProjectID string `json:"projectId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Expired || result.ProjectID != strings.TrimSpace(projectID) {
		return fmt.Errorf("share expiry fixture did not confirm project %q", projectID)
	}
	return nil
}
