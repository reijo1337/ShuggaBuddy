package nightscout

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEntriesSuccess(t *testing.T) {
	entries := []Entry{
		{ID: "1", SGV: 120, Direction: "Flat", DateMs: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC).UnixMilli(), Type: "sgv"},
		{ID: "2", SGV: 180, Direction: "SingleUp", DateMs: time.Date(2026, 4, 18, 12, 5, 0, 0, time.UTC).UnixMilli(), Type: "sgv"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/entries.json", r.URL.Path)
		assert.Equal(t, "sgv", r.URL.Query().Get("find[type]"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", 42)
	result, err := c.GetEntries(context.Background(), time.Time{}, 288)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(42), result[0].UserID)
	assert.InDelta(t, 120.0/MgdlToMmol, result[0].ValueMmol, 0.01)
	assert.Equal(t, "nightscout", result[0].Source)
	require.NotNil(t, result[0].Trend)
	assert.Equal(t, "Flat", *result[0].Trend)
	assert.Equal(t, int64(42), result[1].UserID)
	require.NotNil(t, result[1].Trend)
	assert.Equal(t, "SingleUp", *result[1].Trend)
}

func TestGetEntriesFiltersSGVZero(t *testing.T) {
	entries := []Entry{
		{ID: "1", SGV: 120, Direction: "Flat", DateMs: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC).UnixMilli(), Type: "sgv"},
		{ID: "2", SGV: 0, Direction: "Flat", DateMs: time.Date(2026, 4, 18, 12, 5, 0, 0, time.UTC).UnixMilli(), Type: "sgv"},
		{ID: "3", SGV: -1, Direction: "Flat", DateMs: time.Date(2026, 4, 18, 12, 10, 0, 0, time.UTC).UnixMilli(), Type: "sgv"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", 1)
	result, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.InDelta(t, 120.0/MgdlToMmol, result[0].ValueMmol, 0.01)
}

func TestGetEntriesAuthSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("api-secret")
		assert.NotEmpty(t, secret, "api-secret header should be set")
		assert.Len(t, secret, 40, "should be SHA1 hex")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Entry{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "myapisecret", 1)
	_, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.NoError(t, err)
}

func TestGetEntriesAuthToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		assert.Equal(t, "reader-abc123def", token)
		assert.Empty(t, r.Header.Get("api-secret"), "should not set api-secret when using token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Entry{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "reader-abc123def", 1)
	_, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.NoError(t, err)
}

func TestGetEntriesUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-secret", 1)
	_, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestVerifyConnectionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/status.json", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", 1)
	err := c.VerifyConnection(context.Background())
	assert.NoError(t, err)
}

func TestVerifyConnectionNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-secret", 1)
	err := c.VerifyConnection(context.Background())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestEntryToGlucoseReading(t *testing.T) {
	entry := Entry{
		SGV:       120,
		Direction: "FortyFiveUp",
		DateMs:    time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC).UnixMilli(),
	}

	reading := entry.ToGlucoseReading(42)

	assert.Equal(t, int64(42), reading.UserID)
	assert.InDelta(t, 120.0/18.0182, reading.ValueMmol, 0.01)
	assert.Equal(t, "nightscout", reading.Source)
	require.NotNil(t, reading.Trend)
	assert.Equal(t, "FortyFiveUp", *reading.Trend)
	assert.Equal(t, time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC).UnixMilli(), reading.RecordedAt.UnixMilli())
}

func TestEntryToGlucoseReadingUnknownTrend(t *testing.T) {
	entry := Entry{SGV: 100, Direction: "", DateMs: time.Now().UnixMilli()}
	reading := entry.ToGlucoseReading(1)
	assert.Nil(t, reading.Trend)
}

func TestNormalizeTrend(t *testing.T) {
	tests := []struct {
		input    string
		expected *string
	}{
		{"Flat", strPtr("Flat")},
		{"SingleUp", strPtr("SingleUp")},
		{"DoubleUp", strPtr("DoubleUp")},
		{"FortyFiveUp", strPtr("FortyFiveUp")},
		{"SingleDown", strPtr("SingleDown")},
		{"DoubleDown", strPtr("DoubleDown")},
		{"FortyFiveDown", strPtr("FortyFiveDown")},
		{"NOT COMPUTABLE", nil},
		{"RATE OUT OF RANGE", nil},
		{"None", nil},
		{"", nil},
		{"unknown_value", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTrend(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestMgdlToMmolConversion(t *testing.T) {
	mmol := float64(100) / MgdlToMmol
	assert.InDelta(t, 5.55, mmol, 0.01)

	mmol2 := float64(180) / MgdlToMmol
	assert.InDelta(t, 9.99, mmol2, 0.01)

	// Verify constant consistency with domain.MmolToMgdl
	assert.InDelta(t, 18.0182, MgdlToMmol, 0.001)
	assert.True(t, math.Abs(1.0-MgdlToMmol/18.0182) < 0.001)
}

func strPtr(s string) *string { return &s }
