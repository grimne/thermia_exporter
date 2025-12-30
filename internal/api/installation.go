package api

import (
	"context"
	"encoding/json"
	"fmt"

	"thermia_exporter/internal/types"
)

// GetInstallations retrieves all heat pump installations for the authenticated user.
func (c *APIClient) GetInstallations(ctx context.Context) ([]types.Installation, error) {
	data, err := c.doRequest(ctx, "GET", "/api/v1/installationsInfo", nil)
	if err != nil {
		return nil, err
	}

	// Try parsing as wrapped response first
	var wrap struct {
		Items []types.Installation `json:"items"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil && len(wrap.Items) > 0 {
		return wrap.Items, nil
	}

	// Try parsing as direct array
	var installations []types.Installation
	if err := json.Unmarshal(data, &installations); err == nil {
		return installations, nil
	}

	// Return empty list if no installations found
	return []types.Installation{}, nil
}

// GetInstallationInfo retrieves detailed information about a specific installation.
func (c *APIClient) GetInstallationInfo(ctx context.Context, id int64) (*types.InstallationInfo, error) {
	path := fmt.Sprintf("/api/v1/installations/%d", id)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var info types.InstallationInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal installation info: %w", err)
	}

	return &info, nil
}

// GetInstallationStatus retrieves current status (temperatures, etc.) for an installation.
func (c *APIClient) GetInstallationStatus(ctx context.Context, id int64) (*types.InstallationStatus, error) {
	path := fmt.Sprintf("/api/v1/installationstatus/%d/status", id)

	data, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var status types.InstallationStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal installation status: %w", err)
	}

	return &status, nil
}
