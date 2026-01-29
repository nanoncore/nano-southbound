package common

import "strconv"

// GetAnnotationString retrieves a string value from annotations with optional fallback keys.
// Keys are checked in order - first match wins.
// Returns the value and true if found, empty string and false otherwise.
func GetAnnotationString(annotations map[string]string, keys ...string) (string, bool) {
	if annotations == nil {
		return "", false
	}
	for _, key := range keys {
		if value, ok := annotations[key]; ok {
			return value, true
		}
	}
	return "", false
}

// GetAnnotationInt retrieves an integer value from annotations with optional fallback keys.
// Keys are checked in order - first match wins.
// Returns the value and true if found, 0 and false otherwise.
func GetAnnotationInt(annotations map[string]string, keys ...string) (int, bool) {
	if annotations == nil {
		return 0, false
	}
	for _, key := range keys {
		if valueStr, ok := annotations[key]; ok {
			if value, err := strconv.Atoi(valueStr); err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

// GetAnnotationStringWithDefault retrieves a string from annotations, or returns defaultValue.
// Keys are checked in order - first match wins.
func GetAnnotationStringWithDefault(annotations map[string]string, defaultValue string, keys ...string) string {
	if value, ok := GetAnnotationString(annotations, keys...); ok {
		return value
	}
	return defaultValue
}

// GetAnnotationIntWithDefault retrieves an integer from annotations, or returns defaultValue.
// Keys are checked in order - first match wins.
func GetAnnotationIntWithDefault(annotations map[string]string, defaultValue int, keys ...string) int {
	if value, ok := GetAnnotationInt(annotations, keys...); ok {
		return value
	}
	return defaultValue
}
