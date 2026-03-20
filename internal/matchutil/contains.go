// Package matchutil provides common utilities for stub request matching.
package matchutil

import "reflect"

// MapContains reports whether full contains all key-value pairs from subset, recursively for nested maps.
func MapContains(full, subset map[string]any) bool {
	for k, sv := range subset {
		fv, ok := full[k]
		if !ok {
			return false
		}
		svMap, svIsMap := sv.(map[string]any)
		fvMap, fvIsMap := fv.(map[string]any)
		if svIsMap && fvIsMap {
			if !MapContains(fvMap, svMap) {
				return false
			}
		} else if !reflect.DeepEqual(fv, sv) {
			return false
		}
	}
	return true
}
