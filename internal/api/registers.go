package api

import (
	"context"
	"encoding/json"
	"fmt"

	"thermia_exporter/internal/types"
)

// GetRegisterGroup retrieves a specific register group for an installation.
// Register groups contain configuration and operational data.
func (c *APIClient) GetRegisterGroup(ctx context.Context, installationID int64, group string) ([]types.GroupItem, error) {
	path := fmt.Sprintf("/api/v1/Registers/Installations/%d/Groups/%s", installationID, group)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var items []types.GroupItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("unmarshal register group: %w", err)
	}

	return items, nil
}
