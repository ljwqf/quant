package dataservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEconomicCalendarClientNilConfig(t *testing.T) {
	client := NewEconomicCalendarClient(nil)
	require.NotNil(t, client)
	assert.Equal(t, 7, client.config.DaysAhead)
	assert.Equal(t, 15*time.Second, client.config.Timeout)
}

func TestNewEconomicCalendarClientCustomConfig(t *testing.T) {
	cfg := &EconomicCalendarConfig{
		BaseURL:   "https://custom.api/calendar",
		Countries: []string{"US", "CN"},
		DaysAhead: 14,
		APIKey:    "test-key",
	}
	client := NewEconomicCalendarClient(cfg)
	require.NotNil(t, client)
	assert.Equal(t, "https://custom.api/calendar", client.config.BaseURL)
	assert.Equal(t, []string{"US", "CN"}, client.config.Countries)
	assert.Equal(t, 14, client.config.DaysAhead)
	assert.Equal(t, "test-key", client.config.APIKey)
}

func TestEconomicCalendarFetchEvents(t *testing.T) {
	client := NewEconomicCalendarClient(nil)
	events, err := client.FetchEvents()
	// Without API key, should return built-in events (no error)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	// All events should have valid titles and countries
	for _, e := range events {
		assert.NotEmpty(t, e.Title)
		assert.NotEmpty(t, e.Country)
		assert.GreaterOrEqual(t, e.Importance, 1)
		assert.LessOrEqual(t, e.Importance, 5)
	}
}

func TestEconomicCalendarCaching(t *testing.T) {
	client := NewEconomicCalendarClient(nil)

	// First fetch
	events1, err := client.FetchEvents()
	require.NoError(t, err)

	// Second fetch should return cached copy
	events2, err := client.FetchEvents()
	require.NoError(t, err)

	// Should be the same length
	assert.Equal(t, len(events1), len(events2))
}

func TestGetBuiltInEventsCount(t *testing.T) {
	client := NewEconomicCalendarClient(nil)
	events := client.getBuiltInEvents()
	// Should have 6 built-in events
	assert.Len(t, events, 6)
}

func TestGetBuiltInEventsDates(t *testing.T) {
	client := NewEconomicCalendarClient(nil)
	events := client.getBuiltInEvents()

	for _, e := range events {
		// Events should be scheduled within a reasonable range (-7 to +15 days from now)
		// The weekday math can produce slightly past dates depending on the day of week
		assert.True(t, e.EventTime.After(time.Now().Add(-8*24*time.Hour)),
			"event %s date too far in past: %v", e.Title, e.EventTime)
		assert.True(t, e.EventTime.Before(time.Now().Add(15*24*time.Hour)),
			"event %s date too far in future: %v", e.Title, e.EventTime)
	}
}

func TestGetBuiltInEventsImportance(t *testing.T) {
	client := NewEconomicCalendarClient(nil)
	events := client.getBuiltInEvents()

	for _, e := range events {
		assert.GreaterOrEqual(t, e.Importance, 1)
		assert.LessOrEqual(t, e.Importance, 5)
	}
}

func TestParseFloatString(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"", 0},
		{"-", 0},
		{"100.5", 100.5},
		{"-5.2", -5.2},
		{"3%", 3},
		{"  123  ", 123},
		{"0.00", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.InDelta(t, tt.expected, parseFloatString(tt.input), 0.0001)
		})
	}
}
