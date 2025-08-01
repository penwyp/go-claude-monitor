package util

import (
	"sync"
	"time"
)

// TimeProvider is a global time utility that handles timezone-aware time operations
type TimeProvider struct {
	location *time.Location
	mu       sync.RWMutex
}

var (
	globalTimeProvider *TimeProvider
	once               sync.Once
)

// InitializeTimeProvider initializes the global time provider with the specified timezone
func InitializeTimeProvider(timezone string) error {
	var err error
	once.Do(func() {
		globalTimeProvider = &TimeProvider{}
		err = globalTimeProvider.SetTimezone(timezone)
	})
	return err
}

// GetTimeProvider returns the global time provider instance
// If not initialized, it defaults to Local timezone
func GetTimeProvider() *TimeProvider {
	if globalTimeProvider == nil {
		InitializeTimeProvider("Local")
	}
	return globalTimeProvider
}

// SetTimezone updates the timezone for the time provider
func (tp *TimeProvider) SetTimezone(timezone string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	loc := time.Local
	if timezone != "" && timezone != "Local" {
		l, err := time.LoadLocation(timezone)
		if err != nil {
			return err
		}
		loc = l
	}
	tp.location = loc
	return nil
}

// Now returns the current time in the configured timezone
func (tp *TimeProvider) Now() time.Time {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return time.Now().In(tp.location)
}

// In converts a time to the configured timezone
func (tp *TimeProvider) In(t time.Time) time.Time {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return t.In(tp.location)
}

// Format formats a time according to the layout in the configured timezone
func (tp *TimeProvider) Format(t time.Time, layout string) string {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return t.In(tp.location).Format(layout)
}

// FormatNow formats the current time according to the layout
func (tp *TimeProvider) FormatNow(layout string) string {
	return tp.Format(time.Now(), layout)
}
