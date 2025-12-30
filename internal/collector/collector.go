// Package collector implements the Prometheus collector interface for Thermia heat pumps.
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"thermia_exporter/internal/api"
	"thermia_exporter/internal/auth"
	"thermia_exporter/internal/mapper"
	"thermia_exporter/internal/types"
)

// ThermiaCollector implements prometheus.Collector for Thermia heat pumps.
type ThermiaCollector struct {
	authClient *auth.AuthClient
	creds      auth.Credentials
	logger     *slog.Logger
	metrics    *MetricSet

	// Token cache to minimize login attempts
	tokenCache     *auth.AuthResult
	tokenCacheMu   sync.RWMutex
	tokenExpiresAt time.Time
}

// NewThermiaCollector creates a new Thermia collector.
func NewThermiaCollector(authClient *auth.AuthClient, creds auth.Credentials, logger *slog.Logger) *ThermiaCollector {
	return &ThermiaCollector{
		authClient: authClient,
		creds:      creds,
		logger:     logger,
		metrics:    newMetricSet(),
	}
}

// getOrRefreshToken returns a cached token if valid, or authenticates to get a new one.
// This minimizes login attempts to avoid raising concerns with the heat pump manufacturer.
func (c *ThermiaCollector) getOrRefreshToken(ctx context.Context) (*auth.AuthResult, error) {
	// Try to use cached token first
	c.tokenCacheMu.RLock()
	if c.tokenCache != nil && time.Now().Before(c.tokenExpiresAt) {
		c.logger.Debug("Using cached authentication token",
			"expires_in", time.Until(c.tokenExpiresAt).Round(time.Second))
		token := c.tokenCache
		c.tokenCacheMu.RUnlock()
		return token, nil
	}
	c.tokenCacheMu.RUnlock()

	// Token expired or missing - authenticate
	c.tokenCacheMu.Lock()
	defer c.tokenCacheMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if c.tokenCache != nil && time.Now().Before(c.tokenExpiresAt) {
		c.logger.Debug("Using cached token (acquired after lock)")
		return c.tokenCache, nil
	}

	// Perform authentication
	c.logger.Info("Authenticating to Thermia API", "reason", "token expired or missing")
	authResult, err := c.authClient.Authenticate(ctx, c.creds)
	if err != nil {
		return nil, err
	}

	// Cache the token
	c.tokenCache = authResult
	// Set expiration to 5 minutes before actual expiry for safety margin
	expiresIn := time.Duration(authResult.ExpiresIn) * time.Second
	if expiresIn > 5*time.Minute {
		expiresIn -= 5 * time.Minute
	}
	c.tokenExpiresAt = time.Now().Add(expiresIn)

	c.logger.Info("Authentication successful, token cached",
		"expires_in", expiresIn.Round(time.Second))

	return authResult, nil
}

// Describe implements prometheus.Collector.
func (c *ThermiaCollector) Describe(ch chan<- *prometheus.Desc) {
	// Temperature metrics
	ch <- c.metrics.indoorTemp
	ch <- c.metrics.outdoorTemp
	ch <- c.metrics.supplyLineTemp
	ch <- c.metrics.desiredSupplyTemp
	ch <- c.metrics.returnLineTemp
	ch <- c.metrics.bufferTankTemp
	ch <- c.metrics.hotWaterTemp
	ch <- c.metrics.brineOutTemp
	ch <- c.metrics.brineInTemp
	ch <- c.metrics.poolTemp
	ch <- c.metrics.coolingTankTemp
	ch <- c.metrics.coolingSupplyTemp

	// Status metrics
	ch <- c.metrics.online
	ch <- c.metrics.lastOnlineUnix

	// Mode/status metrics
	ch <- c.metrics.operationMode
	ch <- c.metrics.operationModeAvail
	ch <- c.metrics.operationalStatus
	ch <- c.metrics.operationalStatusAvail
	ch <- c.metrics.powerStatus
	ch <- c.metrics.powerStatusAvail

	// Hot water metrics
	ch <- c.metrics.hotWaterSwitch
	ch <- c.metrics.hotWaterBoost

	// Operational time metrics
	ch <- c.metrics.operTimeCompressor
	ch <- c.metrics.operTimeHeating
	ch <- c.metrics.operTimeHotWater
	ch <- c.metrics.operTimeImm1
	ch <- c.metrics.operTimeImm2
	ch <- c.metrics.operTimeImm3

	// Alert metrics
	ch <- c.metrics.activeAlerts
	ch <- c.metrics.archivedAlerts

	// Scrape metrics
	c.metrics.scrapeErrors.Describe(ch)
	c.metrics.scrapeDuration.Describe(ch)
}

// Collect implements prometheus.Collector.
// It performs on-demand scraping when Prometheus scrapes the /metrics endpoint.
func (c *ThermiaCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		c.metrics.scrapeDuration.Observe(duration)
		c.metrics.scrapeDuration.Collect(ch)
	}()

	// Get or refresh authentication token
	c.logger.Debug("Starting scrape")
	authResult, err := c.getOrRefreshToken(ctx)
	if err != nil {
		c.metrics.scrapeErrors.Inc()
		c.metrics.scrapeErrors.Collect(ch)
		c.logger.Error("Authentication failed during scrape", "error", err)
		return
	}

	// Create API client
	apiClient, err := api.NewAPIClient(ctx, authResult.AccessToken, c.logger)
	if err != nil {
		c.metrics.scrapeErrors.Inc()
		c.metrics.scrapeErrors.Collect(ch)
		c.logger.Error("Failed to create API client", "error", err)
		return
	}

	// Get installations
	installations, err := apiClient.GetInstallations(ctx)
	if err != nil {
		c.metrics.scrapeErrors.Inc()
		c.metrics.scrapeErrors.Collect(ch)
		c.logger.Error("Failed to get installations", "error", err)
		return
	}

	if len(installations) == 0 {
		c.logger.Warn("No installations found")
		c.metrics.scrapeErrors.Collect(ch)
		return
	}

	// Collect metrics for the first installation (as per requirements)
	c.collectInstallation(ctx, ch, apiClient, installations[0])
	c.metrics.scrapeErrors.Collect(ch)
}

// collectInstallation collects all metrics for a single installation.
func (c *ThermiaCollector) collectInstallation(ctx context.Context, ch chan<- prometheus.Metric, apiClient *api.APIClient, inst types.Installation) {
	// Fetch installation info
	info, err := apiClient.GetInstallationInfo(ctx, inst.ID)
	if err != nil {
		c.logger.Error("Failed to get installation info", "id", inst.ID, "error", err)
		return
	}

	// Fetch installation status
	status, err := apiClient.GetInstallationStatus(ctx, inst.ID)
	if err != nil {
		c.logger.Error("Failed to get installation status", "id", inst.ID, "error", err)
		return
	}

	// Fetch register groups (with error logging, but continue with partial data)
	grpOperation, err := apiClient.GetRegisterGroup(ctx, inst.ID, mapper.RegGroupOperationalOperation)
	if err != nil {
		c.logger.Warn("Failed to get operation registers", "id", inst.ID, "error", err)
	}

	grpStatus, err := apiClient.GetRegisterGroup(ctx, inst.ID, mapper.RegGroupOperationalStatus)
	if err != nil {
		c.logger.Warn("Failed to get status registers", "id", inst.ID, "error", err)
	}

	grpTemps, err := apiClient.GetRegisterGroup(ctx, inst.ID, mapper.RegGroupTemperatures)
	if err != nil {
		c.logger.Warn("Failed to get temperature registers", "id", inst.ID, "error", err)
	}

	grpTime, err := apiClient.GetRegisterGroup(ctx, inst.ID, mapper.RegGroupOperationalTime)
	if err != nil {
		c.logger.Warn("Failed to get operational time registers", "id", inst.ID, "error", err)
	}

	grpHot, err := apiClient.GetRegisterGroup(ctx, inst.ID, mapper.RegGroupHotWater)
	if err != nil {
		c.logger.Warn("Failed to get hot water registers", "id", inst.ID, "error", err)
	}

	// Fetch events/alerts
	activeEvents, err := apiClient.GetEvents(ctx, inst.ID, true)
	if err != nil {
		c.logger.Warn("Failed to get active events", "id", inst.ID, "error", err)
	}

	allEvents, err := apiClient.GetEvents(ctx, inst.ID, false)
	if err != nil {
		c.logger.Warn("Failed to get all events", "id", inst.ID, "error", err)
	}

	// Build base labels
	model := mapper.Safe(info.Model, info.Profile.Name)
	labels := []string{
		fmt.Sprint(inst.ID),
		mapper.Safe(info.Name, inst.Name),
		model,
	}

	// Extract and emit metrics
	c.emitTemperatureMetrics(ch, labels, status, grpTemps)
	c.emitStatusMetrics(ch, labels, info)
	c.emitModeMetrics(ch, labels, grpOperation)
	c.emitOperationalStatusMetrics(ch, labels, grpStatus)
	c.emitPowerStatusMetrics(ch, labels, grpStatus)
	c.emitHotWaterMetrics(ch, labels, grpHot)
	c.emitOperationalTimeMetrics(ch, labels, grpTime)
	c.emitAlertMetrics(ch, labels, activeEvents, allEvents)
}

// emitTemperatureMetrics emits all temperature metrics.
func (c *ThermiaCollector) emitTemperatureMetrics(ch chan<- prometheus.Metric, labels []string, status *types.InstallationStatus, grpTemps []types.GroupItem) {
	temps := mapper.ExtractTemperatures(status, grpTemps)

	// Also get outdoor temp from registers
	if outdoor := mapper.FindValue(grpTemps, mapper.RegOutdoorTemperature); outdoor == nil {
		temps.Outdoor = mapper.FindValue(grpTemps, mapper.RegOperDataOutdoorTempMaSa)
	} else {
		temps.Outdoor = outdoor
	}

	tempMap := mapper.TemperaturesToMap(temps)

	tempDescs := map[string]*prometheus.Desc{
		"indoor":              c.metrics.indoorTemp,
		"outdoor":             c.metrics.outdoorTemp,
		"supply_line":         c.metrics.supplyLineTemp,
		"desired_supply_line": c.metrics.desiredSupplyTemp,
		"return_line":         c.metrics.returnLineTemp,
		"buffer_tank":         c.metrics.bufferTankTemp,
		"hot_water":           c.metrics.hotWaterTemp,
		"brine_out":           c.metrics.brineOutTemp,
		"brine_in":            c.metrics.brineInTemp,
		"pool":                c.metrics.poolTemp,
		"cooling_tank":        c.metrics.coolingTankTemp,
		"cooling_supply":      c.metrics.coolingSupplyTemp,
	}

	for name, value := range tempMap {
		if desc, ok := tempDescs[name]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
		}
	}
}

// emitStatusMetrics emits online status metrics.
func (c *ThermiaCollector) emitStatusMetrics(ch chan<- prometheus.Metric, labels []string, info *types.InstallationInfo) {
	onlineValue := 0.0
	if info.IsOnline {
		onlineValue = 1.0
	}
	ch <- prometheus.MustNewConstMetric(c.metrics.online, prometheus.GaugeValue, onlineValue, labels...)

	if lastOnlineUnix := mapper.ParseTimeToUnix(info.LastOnline); lastOnlineUnix > 0 {
		ch <- prometheus.MustNewConstMetric(c.metrics.lastOnlineUnix, prometheus.GaugeValue, float64(lastOnlineUnix), labels...)
	}
}

// emitModeMetrics emits operation mode metrics.
func (c *ThermiaCollector) emitModeMetrics(ch chan<- prometheus.Metric, labels []string, grpOperation []types.GroupItem) {
	modeData := mapper.ExtractOperationMode(grpOperation)

	// Available modes
	for _, mode := range modeData.Available {
		labelsWithMode := append(labels, mode)
		ch <- prometheus.MustNewConstMetric(c.metrics.operationModeAvail, prometheus.GaugeValue, 1, labelsWithMode...)
	}

	// Current mode
	if modeData.Current != "" {
		labelsWithMode := append(labels, modeData.Current)
		ch <- prometheus.MustNewConstMetric(c.metrics.operationMode, prometheus.GaugeValue, 1, labelsWithMode...)
	}
}

// emitOperationalStatusMetrics emits operational status metrics.
func (c *ThermiaCollector) emitOperationalStatusMetrics(ch chan<- prometheus.Metric, labels []string, grpStatus []types.GroupItem) {
	statusData := mapper.ExtractBitmaskStatuses(grpStatus, mapper.OperationalStatusCandidates)

	// Available statuses
	for _, status := range statusData.Available {
		labelsWithStatus := append(labels, status)
		ch <- prometheus.MustNewConstMetric(c.metrics.operationalStatusAvail, prometheus.GaugeValue, 1, labelsWithStatus...)
	}

	// Running statuses (one-hot encoding - pick primary status)
	runningSet := make(map[string]bool)
	for _, s := range statusData.Running {
		runningSet[strings.ToUpper(s)] = true
	}

	current := pickCurrentStatus(statusData.Running, statusData.Available)
	for _, status := range statusData.Available {
		value := 0.0
		if strings.EqualFold(status, current) {
			value = 1.0
		}
		labelsWithStatus := append(labels, status)
		ch <- prometheus.MustNewConstMetric(c.metrics.operationalStatus, prometheus.GaugeValue, value, labelsWithStatus...)
	}
}

// emitPowerStatusMetrics emits power status metrics.
func (c *ThermiaCollector) emitPowerStatusMetrics(ch chan<- prometheus.Metric, labels []string, grpStatus []types.GroupItem) {
	powerData := mapper.ExtractBitmaskStatuses(grpStatus, mapper.PowerStatusCandidates)

	// Available power statuses
	for _, status := range powerData.Available {
		labelsWithStatus := append(labels, status)
		ch <- prometheus.MustNewConstMetric(c.metrics.powerStatusAvail, prometheus.GaugeValue, 1, labelsWithStatus...)
	}

	// Running power statuses (can be multiple)
	runningSet := make(map[string]bool)
	for _, s := range powerData.Running {
		runningSet[s] = true
	}

	for _, status := range powerData.Available {
		value := 0.0
		if runningSet[status] {
			value = 1.0
		}
		labelsWithStatus := append(labels, status)
		ch <- prometheus.MustNewConstMetric(c.metrics.powerStatus, prometheus.GaugeValue, value, labelsWithStatus...)
	}
}

// emitHotWaterMetrics emits hot water switch and boost metrics.
func (c *ThermiaCollector) emitHotWaterMetrics(ch chan<- prometheus.Metric, labels []string, grpHot []types.GroupItem) {
	switchState, boostState := mapper.ExtractHotWaterSwitches(grpHot)

	if switchState != nil {
		ch <- prometheus.MustNewConstMetric(c.metrics.hotWaterSwitch, prometheus.GaugeValue, float64(*switchState), labels...)
	}

	if boostState != nil {
		ch <- prometheus.MustNewConstMetric(c.metrics.hotWaterBoost, prometheus.GaugeValue, float64(*boostState), labels...)
	}
}

// emitOperationalTimeMetrics emits operational time counter metrics.
func (c *ThermiaCollector) emitOperationalTimeMetrics(ch chan<- prometheus.Metric, labels []string, grpTime []types.GroupItem) {
	opTime := mapper.ExtractOperationalTime(grpTime)

	timeDescs := map[string]*prometheus.Desc{
		mapper.RegOperTimeCompressor: c.metrics.operTimeCompressor,
		mapper.RegOperTimeHeating:    c.metrics.operTimeHeating,
		mapper.RegOperTimeHotWater:   c.metrics.operTimeHotWater,
		mapper.RegOperTimeImm1:       c.metrics.operTimeImm1,
		mapper.RegOperTimeImm2:       c.metrics.operTimeImm2,
		mapper.RegOperTimeImm3:       c.metrics.operTimeImm3,
	}

	for regName, hours := range opTime {
		if desc, ok := timeDescs[regName]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(hours), labels...)
		}
	}
}

// emitAlertMetrics emits alert count metrics.
func (c *ThermiaCollector) emitAlertMetrics(ch chan<- prometheus.Metric, labels []string, activeEvents, allEvents []types.Event) {
	active, archived := mapper.ExtractAlerts(activeEvents, allEvents)

	ch <- prometheus.MustNewConstMetric(c.metrics.activeAlerts, prometheus.GaugeValue, float64(len(active)), labels...)
	ch <- prometheus.MustNewConstMetric(c.metrics.archivedAlerts, prometheus.GaugeValue, float64(len(archived)), labels...)
}

// pickCurrentStatus chooses the most relevant operational status from running statuses.
// This matches the logic from the original implementation.
func pickCurrentStatus(running, available []string) string {
	if len(running) == 0 && len(available) == 0 {
		return ""
	}

	// Filter out NO_DEMAND if other statuses are present
	filtered := make([]string, 0, len(running))
	for _, s := range running {
		if !strings.EqualFold(s, "STATUS_NO_DEMAND") {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		filtered = running
	}

	// Priority order (highest first)
	priority := []string{
		"STATUS_LEGIONELLA",
		"STATUS_HOTWATER",
		"STATUS_HEAT",
		"STATUS_COOL",
		"STATUS_PASSIVE_COOL",
		"STATUS_POOL",
		"STATUS_STANDBY",
		"STATUS_NO_DEMAND",
		"OPERATION_MODE_OFF",
	}

	have := make(map[string]struct{})
	for _, s := range filtered {
		have[strings.ToUpper(s)] = struct{}{}
	}

	for _, p := range priority {
		if _, ok := have[p]; ok {
			return p
		}
	}

	// Fallback: first running, or first available
	if len(filtered) > 0 {
		return filtered[0]
	}
	if len(available) > 0 {
		return available[0]
	}

	return ""
}
