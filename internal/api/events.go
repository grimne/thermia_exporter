package api

import (
	"context"
	"encoding/json"
	"fmt"

	"thermia_exporter/internal/types"
)

// GetEvents retrieves events/alarms for an installation.
// If onlyActive is true, returns only currently active alarms.
// If onlyActive is false, returns all alarms (active and historical).
func (c *APIClient) GetEvents(ctx context.Context, installationID int64, onlyActive bool) ([]types.Event, error) {
	path := fmt.Sprintf("/api/v1/installation/%d/events?onlyActiveAlarms=%v", installationID, onlyActive)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var events []types.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("unmarshal events: %w", err)
	}

	return events, nil
}
