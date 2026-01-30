package common

import "testing"

func TestGetAnnotationString(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		keys        []string
		wantValue   string
		wantFound   bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			keys:        []string{"key1"},
			wantValue:   "",
			wantFound:   false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			keys:        []string{"key1"},
			wantValue:   "",
			wantFound:   false,
		},
		{
			name:        "key found",
			annotations: map[string]string{"key1": "value1"},
			keys:        []string{"key1"},
			wantValue:   "value1",
			wantFound:   true,
		},
		{
			name:        "key not found",
			annotations: map[string]string{"key1": "value1"},
			keys:        []string{"key2"},
			wantValue:   "",
			wantFound:   false,
		},
		{
			name:        "fallback key found",
			annotations: map[string]string{"key2": "value2"},
			keys:        []string{"key1", "key2"},
			wantValue:   "value2",
			wantFound:   true,
		},
		{
			name:        "first key wins",
			annotations: map[string]string{"key1": "value1", "key2": "value2"},
			keys:        []string{"key1", "key2"},
			wantValue:   "value1",
			wantFound:   true,
		},
		{
			name:        "empty value is valid",
			annotations: map[string]string{"key1": ""},
			keys:        []string{"key1"},
			wantValue:   "",
			wantFound:   true,
		},
		{
			name:        "no keys provided",
			annotations: map[string]string{"key1": "value1"},
			keys:        []string{},
			wantValue:   "",
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := GetAnnotationString(tt.annotations, tt.keys...)
			if gotValue != tt.wantValue {
				t.Errorf("GetAnnotationString() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetAnnotationString() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestGetAnnotationInt(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		keys        []string
		wantValue   int
		wantFound   bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			keys:        []string{"key1"},
			wantValue:   0,
			wantFound:   false,
		},
		{
			name:        "valid integer",
			annotations: map[string]string{"key1": "42"},
			keys:        []string{"key1"},
			wantValue:   42,
			wantFound:   true,
		},
		{
			name:        "invalid integer",
			annotations: map[string]string{"key1": "not-a-number"},
			keys:        []string{"key1"},
			wantValue:   0,
			wantFound:   false,
		},
		{
			name:        "fallback to valid integer",
			annotations: map[string]string{"key1": "invalid", "key2": "123"},
			keys:        []string{"key1", "key2"},
			wantValue:   123,
			wantFound:   true,
		},
		{
			name:        "negative integer",
			annotations: map[string]string{"key1": "-5"},
			keys:        []string{"key1"},
			wantValue:   -5,
			wantFound:   true,
		},
		{
			name:        "zero value",
			annotations: map[string]string{"key1": "0"},
			keys:        []string{"key1"},
			wantValue:   0,
			wantFound:   true,
		},
		{
			name:        "empty string is invalid",
			annotations: map[string]string{"key1": ""},
			keys:        []string{"key1"},
			wantValue:   0,
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := GetAnnotationInt(tt.annotations, tt.keys...)
			if gotValue != tt.wantValue {
				t.Errorf("GetAnnotationInt() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetAnnotationInt() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestGetAnnotationStringWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		defaultValue string
		keys         []string
		want         string
	}{
		{
			name:         "returns default on nil",
			annotations:  nil,
			defaultValue: "default",
			keys:         []string{"key1"},
			want:         "default",
		},
		{
			name:         "returns value when found",
			annotations:  map[string]string{"key1": "value1"},
			defaultValue: "default",
			keys:         []string{"key1"},
			want:         "value1",
		},
		{
			name:         "returns default when not found",
			annotations:  map[string]string{"key1": "value1"},
			defaultValue: "default",
			keys:         []string{"key2"},
			want:         "default",
		},
		{
			name:         "empty value overrides default",
			annotations:  map[string]string{"key1": ""},
			defaultValue: "default",
			keys:         []string{"key1"},
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAnnotationStringWithDefault(tt.annotations, tt.defaultValue, tt.keys...)
			if got != tt.want {
				t.Errorf("GetAnnotationStringWithDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAnnotationIntWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		defaultValue int
		keys         []string
		want         int
	}{
		{
			name:         "returns default on nil",
			annotations:  nil,
			defaultValue: 99,
			keys:         []string{"key1"},
			want:         99,
		},
		{
			name:         "returns value when found",
			annotations:  map[string]string{"key1": "42"},
			defaultValue: 99,
			keys:         []string{"key1"},
			want:         42,
		},
		{
			name:         "returns default on invalid int",
			annotations:  map[string]string{"key1": "invalid"},
			defaultValue: 99,
			keys:         []string{"key1"},
			want:         99,
		},
		{
			name:         "fallback key with valid int",
			annotations:  map[string]string{"key2": "123"},
			defaultValue: 99,
			keys:         []string{"key1", "key2"},
			want:         123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAnnotationIntWithDefault(tt.annotations, tt.defaultValue, tt.keys...)
			if got != tt.want {
				t.Errorf("GetAnnotationIntWithDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
