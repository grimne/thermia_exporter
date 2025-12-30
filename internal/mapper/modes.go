package mapper

import (
	"strings"

	"thermia_exporter/internal/types"
)

// ExtractOperationMode extracts the current and available operation modes from register items.
func ExtractOperationMode(items []types.GroupItem) types.OperationModeData {
	var result types.OperationModeData

	for _, it := range items {
		if it.RegisterName == RegOperationMode {
			result.Available = make([]string, 0, len(it.ValueNames))
			for _, vn := range it.ValueNames {
				if vn.Visible {
					result.Available = append(result.Available, trimMode(vn.Name))
				}
			}
			result.ReadOnly = it.IsReadOnly

			if it.RegisterValue != nil {
				val := int(*it.RegisterValue + 0.00001)
				for _, vn := range it.ValueNames {
					if vn.Value == val {
						result.Current = trimMode(vn.Name)
						break
					}
				}
			}
			break
		}
	}

	return result
}

// trimMode removes common prefixes from operation mode names.
func trimMode(s string) string {
	s = strings.TrimPrefix(s, ModePrefixRegValue)
	s = strings.TrimPrefix(s, "REG_VALUE_")
	return s
}
