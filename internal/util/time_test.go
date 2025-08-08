package util

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeTimeProvider(t *testing.T) {
	// Reset global provider before tests
	mu.Lock()
	globalTimeProvider = nil
	mu.Unlock()

	tests := []struct {
		name     string
		timezone string
		wantErr  bool
	}{
		{
			name:     "local timezone",
			timezone: "Local",
			wantErr:  false,
		},
		{
			name:     "UTC timezone",
			timezone: "UTC",
			wantErr:  false,
		},
		{
			name:     "valid timezone Asia/Shanghai",
			timezone: "Asia/Shanghai",
			wantErr:  false,
		},
		{
			name:     "valid timezone America/New_York",
			timezone: "America/New_York",
			wantErr:  false,
		},
		{
			name:     "invalid timezone",
			timezone: "Invalid/Timezone",
			wantErr:  true,
		},
		{
			name:     "empty timezone defaults to Local",
			timezone: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitializeTimeProvider(tt.timezone)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid timezone")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, globalTimeProvider)
			}
		})
	}
}

func TestGetTimeProvider(t *testing.T) {
	// Reset global provider
	mu.Lock()
	globalTimeProvider = nil
	mu.Unlock()

	// First call should initialize with Local timezone
	provider := GetTimeProvider()
	assert.NotNil(t, provider)

	// Second call should return the same instance
	provider2 := GetTimeProvider()
	assert.Equal(t, provider, provider2)
}

func TestTimeProvider_SetTimezone(t *testing.T) {
	provider := &TimeProvider{}

	tests := []struct {
		name     string
		timezone string
		wantErr  bool
	}{
		{
			name:     "set to UTC",
			timezone: "UTC",
			wantErr:  false,
		},
		{
			name:     "set to Asia/Tokyo",
			timezone: "Asia/Tokyo",
			wantErr:  false,
		},
		{
			name:     "set to Europe/London",
			timezone: "Europe/London",
			wantErr:  false,
		},
		{
			name:     "set to Local",
			timezone: "Local",
			wantErr:  false,
		},
		{
			name:     "empty string defaults to Local",
			timezone: "",
			wantErr:  false,
		},
		{
			name:     "invalid timezone",
			timezone: "Not/A/Timezone",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.SetTimezone(tt.timezone)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTimeProvider_Now(t *testing.T) {
	provider := &TimeProvider{}
	
	// Set to UTC for predictable testing
	err := provider.SetTimezone("UTC")
	require.NoError(t, err)

	before := time.Now().UTC()
	now := provider.Now()
	after := time.Now().UTC()

	// The provider's Now() should be between before and after
	assert.True(t, now.After(before) || now.Equal(before))
	assert.True(t, now.Before(after) || now.Equal(after))
	
	// Should be in UTC timezone
	assert.Equal(t, "UTC", now.Location().String())
}

func TestTimeProvider_In(t *testing.T) {
	provider := &TimeProvider{}
	
	// Set to Asia/Shanghai
	err := provider.SetTimezone("Asia/Shanghai")
	require.NoError(t, err)

	// Create a UTC time
	utcTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	// Convert to Shanghai time
	shanghaiTime := provider.In(utcTime)
	
	// Should be the same instant but different timezone
	assert.True(t, utcTime.Equal(shanghaiTime))
	assert.Equal(t, "Asia/Shanghai", shanghaiTime.Location().String())
	
	// Shanghai is UTC+8, so should be 20:00
	assert.Equal(t, 20, shanghaiTime.Hour())
}

func TestTimeProvider_Format(t *testing.T) {
	provider := &TimeProvider{}
	
	// Set to UTC for predictable testing
	err := provider.SetTimezone("UTC")
	require.NoError(t, err)

	testTime := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)
	
	tests := []struct {
		name     string
		layout   string
		expected string
	}{
		{
			name:     "RFC3339",
			layout:   time.RFC3339,
			expected: "2024-03-15T14:30:45Z",
		},
		{
			name:     "date only",
			layout:   "2006-01-02",
			expected: "2024-03-15",
		},
		{
			name:     "time only",
			layout:   "15:04:05",
			expected: "14:30:45",
		},
		{
			name:     "custom format",
			layout:   "Jan 2, 2006 at 3:04 PM",
			expected: "Mar 15, 2024 at 2:30 PM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.Format(testTime, tt.layout)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimeProvider_FormatNow(t *testing.T) {
	provider := &TimeProvider{}
	
	// Set to UTC for predictable testing
	err := provider.SetTimezone("UTC")
	require.NoError(t, err)

	// FormatNow should return a non-empty string
	result := provider.FormatNow("2006-01-02 15:04:05")
	assert.NotEmpty(t, result)
	
	// Should match the pattern
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
}

func TestTimeProvider_Concurrency(t *testing.T) {
	provider := &TimeProvider{}
	err := provider.SetTimezone("UTC")
	require.NoError(t, err)

	// Test concurrent access
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = provider.Now()
			_ = provider.In(time.Now())
			_ = provider.Format(time.Now(), time.RFC3339)
			_ = provider.FormatNow(time.RFC3339)
		}()
	}

	// Concurrent timezone changes
	timezones := []string{"UTC", "Asia/Shanghai", "America/New_York", "Europe/London"}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tz := timezones[idx%len(timezones)]
			if err := provider.SetTimezone(tz); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

func TestTimeProvider_TimezoneConversions(t *testing.T) {
	provider := &TimeProvider{}

	// Test time in different timezones
	testTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		timezone     string
		expectedHour int
		expectedDay  int
	}{
		{"UTC", 12, 15},
		{"Asia/Shanghai", 20, 15},        // UTC+8
		{"America/New_York", 8, 15},      // UTC-4 (DST)
		{"Asia/Tokyo", 21, 15},           // UTC+9
		{"America/Los_Angeles", 5, 15},   // UTC-7 (DST)
		{"Australia/Sydney", 22, 15},     // UTC+10
	}

	for _, tt := range tests {
		t.Run(tt.timezone, func(t *testing.T) {
			err := provider.SetTimezone(tt.timezone)
			require.NoError(t, err)

			converted := provider.In(testTime)
			assert.Equal(t, tt.expectedHour, converted.Hour())
			assert.Equal(t, tt.expectedDay, converted.Day())
		})
	}
}

func TestInitializeTimeProvider_ErrorMessage(t *testing.T) {
	// Test that error message includes helpful examples
	err := InitializeTimeProvider("Invalid/Zone")
	require.Error(t, err)
	
	assert.Contains(t, err.Error(), "invalid timezone 'Invalid/Zone'")
	assert.Contains(t, err.Error(), "Valid examples:")
	assert.Contains(t, err.Error(), "America/New_York")
	assert.Contains(t, err.Error(), "Asia/Shanghai")
}