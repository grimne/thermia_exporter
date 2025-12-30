package mapper

import (
	"strings"
	"time"

	"thermia_exporter/internal/types"
)

// ExtractBitmaskStatuses extracts bitmask status flags from register items.
// It searches for the first matching register from the provided register names.
func ExtractBitmaskStatuses(items []types.GroupItem, registerNames []string) types.StatusData {
	var result types.StatusData
	var match *types.GroupItem

	// Find the first matching register
	for _, rn := range registerNames {
		for i := range items {
			if items[i].RegisterName == rn {
				match = &items[i]
				break
			}
		}
		if match != nil {
			break
		}
	}

	if match == nil {
		return result
	}

	// Extract available statuses
	result.Available = make([]string, 0, len(match.ValueNames))
	for _, vn := range match.ValueNames {
		if vn.Visible {
			result.Available = append(result.Available, trimStatus(vn.Name))
		}
	}

	// Extract running statuses (bitmask)
	if match.RegisterValue == nil {
		result.Running = []string{}
		return result
	}

	val := int(*match.RegisterValue + 0.00001)
	result.Running = make([]string, 0)
	for _, vn := range match.ValueNames {
		if vn.Visible && (val&vn.Value) != 0 {
			result.Running = append(result.Running, trimStatus(vn.Name))
		}
	}

	return result
}

// ExtractHotWaterSwitches extracts hot water switch and boost states.
// Returns pointers to int (0 or 1) for each switch, or nil if not found.
func ExtractHotWaterSwitches(items []types.GroupItem) (switchState *int, boostState *int) {
	for _, it := range items {
		switch it.RegisterName {
		case RegHotWaterBoost:
			if it.RegisterValue != nil {
				v := int(*it.RegisterValue + 0.00001)
				boostState = &v
			}
		case RegHotWaterStatus:
			if it.RegisterValue != nil {
				v := int(*it.RegisterValue + 0.00001)
				switchState = &v
			}
		}
	}
	return switchState, boostState
}

// ExtractOperationalTime extracts operational time counters (in hours) from register items.
func ExtractOperationalTime(items []types.GroupItem) map[string]int {
	keys := []string{
		RegOperTimeCompressor,
		RegOperTimeHeating,
		RegOperTimeHotWater,
		RegOperTimeImm1,
		RegOperTimeImm2,
		RegOperTimeImm3,
	}

	result := make(map[string]int)
	for _, k := range keys {
		for _, it := range items {
			if it.RegisterName == k && it.RegisterValue != nil {
				result[k] = int(*it.RegisterValue + 0.00001)
				break
			}
		}
	}

	return result
}

// ExtractAlerts extracts unique alert titles from events and categorizes them.
func ExtractAlerts(activeEvents, allEvents []types.Event) (active, archived []string) {
	activeTitles := uniqueTitles(activeEvents)
	allTitles := uniqueTitles(allEvents)
	archived = difference(allTitles, activeTitles)
	return activeTitles, archived
}

// ParseTimeToUnix converts a time string to Unix timestamp (seconds).
// Supports multiple common time formats. Returns 0 if parsing fails.
func ParseTimeToUnix(s string) int64 {
	if s == "" {
		return 0
	}

	layouts := []string{
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}

	var t time.Time
	var err error
	for _, l := range layouts {
		t, err = time.Parse(l, s)
		if err == nil {
			return t.Unix()
		}
	}

	return 0
}

// trimStatus removes common prefixes from status names.
func trimStatus(s string) string {
	for _, p := range []string{StatusPrefixRegValue, StatusPrefixCompValue} {
		if strings.HasPrefix(s, p) {
			return strings.TrimPrefix(s, p)
		}
	}
	return s
}

// uniqueTitles extracts unique non-empty event titles.
func uniqueTitles(events []types.Event) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, e := range events {
		t := strings.TrimSpace(e.EventTitle)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		result = append(result, t)
	}

	return result
}

// difference returns elements in 'all' that are not in 'active'.
func difference(all, active []string) []string {
	set := make(map[string]struct{}, len(active))
	for _, a := range active {
		set[a] = struct{}{}
	}

	result := make([]string, 0)
	for _, t := range all {
		if _, ok := set[t]; !ok {
			result = append(result, t)
		}
	}

	return result
}

// Safe returns the value if non-empty after trimming, otherwise returns the fallback.
func Safe(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}
