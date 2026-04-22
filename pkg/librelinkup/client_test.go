package librelinkup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/llu/auth/login", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "llu.ios", r.Header.Get("product"))
		assert.Equal(t, appVersion, r.Header.Get("version"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body["email"])
		assert.Equal(t, "pass123", body["password"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResponse{
			Status: 0,
			Data: struct {
				Redirect   bool   `json:"redirect"`
				Region     string `json:"region"`
				AuthTicket struct {
					Token   string `json:"token"`
					Expires int64  `json:"expires"`
				} `json:"authTicket"`
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			}{
				AuthTicket: struct {
					Token   string `json:"token"`
					Expires int64  `json:"expires"`
				}{Token: "test-jwt-token", Expires: 9999999999},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("user@example.com", "pass123", "", 42)
	c.baseURL = srv.URL

	err := c.login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-jwt-token", c.authToken)
}

func TestLoginRedirect(t *testing.T) {
	callCount := 0

	// Regional server (second call).
	regionalSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 0,
			"data": map[string]any{
				"redirect": false,
				"authTicket": map[string]any{
					"token":   "regional-jwt",
					"expires": 9999999999,
				},
			},
		})
	}))
	defer regionalSrv.Close()

	// Primary server (first call) — returns redirect.
	primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 0,
			"data": map[string]any{
				"redirect": true,
				"region":   "de",
			},
		})
	}))
	defer primarySrv.Close()

	c := NewClient("user@example.com", "pass123", "", 42)
	c.baseURL = primarySrv.URL
	// Override baseURLFormat behavior: after redirect detection,
	// the client sets baseURL via fmt.Sprintf(baseURLFormat, region).
	// We need to intercept that, so we'll test the region is saved
	// and then manually set the baseURL to our regional server.
	// Instead, let's test the flow by patching after the first call.

	// For a proper test, we do login which hits primary, gets redirect,
	// then tries regional URL. Since we can't intercept fmt.Sprintf,
	// we test region detection separately and the full flow with a single server.

	// Simpler approach: single server that handles both calls.
	callCount = 0
	bothSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": map[string]any{
					"redirect": true,
					"region":   "de",
				},
			})
			return
		}
		if r.URL.Path == "/llu/config/country" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"regionalMap": map[string]any{
						"de": map[string]any{
							"lslApi": "https://api-de.libreview.io",
						},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 0,
			"data": map[string]any{
				"redirect": false,
				"authTicket": map[string]any{
					"token":   "regional-jwt",
					"expires": 9999999999,
				},
				"user": map[string]any{
					"id": "test-user-id",
				},
			},
		})
	}))
	defer bothSrv.Close()

	c2 := NewClient("user@example.com", "pass123", "", 42)
	c2.baseURL = bothSrv.URL

	// Patch: override the base URL format effect so redirect still hits our test server.
	origLogin := c2.login
	_ = origLogin
	// We can't easily override fmt.Sprintf, so we use a different strategy:
	// after login sets c.baseURL to the regional URL, it will fail to connect.
	// Instead, we override the login method to always use our test server.

	// The cleanest approach: override baseURL in the redirect path.
	// Let's just test that the region is detected and the second call is made.

	// Actually, the simplest way is to make the test server check the path
	// and the client will just call the same server (since we set baseURL).
	// The redirect sets baseURL = fmt.Sprintf(baseURLFormat, region) which
	// won't match our test server. So let's just manually test region detection.

	// Test approach: use a custom login that we can track.
	// After redirect, the client will set baseURL to a non-test URL, which will fail.
	// We verify the region is stored and the first call was made.

	// Better approach: make a server that responds correctly on both calls,
	// and override the client so it stays pointed at our server.

	// Use a wrapper that intercepts the redirect behavior:
	c3 := NewClient("user@example.com", "pass123", "", 42)
	c3.baseURL = bothSrv.URL
	callCount = 0

	// Monkey-patch: after login detects redirect and changes baseURL,
	// we need it to still hit our test server. We do this by making
	// the "regional" URL also point to our test server.
	// Since we can't change the const, let's just test the login behavior
	// by having the server return redirect=false on second call and
	// checking that the client made 2 calls and stored the region.

	// The issue is login() changes baseURL after redirect. To test this properly,
	// we need to intercept that. Let's use a simpler test:
	// set the baseURL manually after the first call returns redirect.

	// Actually the simplest way: just intercept the HTTP transport.
	c3.httpClient = bothSrv.Client()
	c3.httpClient.Transport = &redirectTransport{
		target: bothSrv.URL,
	}

	err := c3.login(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "regional-jwt", c3.authToken)
	assert.Equal(t, "de", c3.region)
	assert.Equal(t, 3, callCount)
}

// redirectTransport redirects all requests to the target server.
type redirectTransport struct {
	target string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := rt.target + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}

func TestLoginUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient("bad@example.com", "wrong", "", 1)
	c.baseURL = srv.URL

	err := c.login(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestGetConnectionsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/llu/connections", r.URL.Path)
		assert.Equal(t, "Bearer test-jwt", r.Header.Get("Authorization"))
		assert.Equal(t, "llu.ios", r.Header.Get("product"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 0,
			"data": []map[string]any{
				{"patientId": "patient-uuid-123"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "p", "", 1)
	c.baseURL = srv.URL
	c.authToken = "test-jwt"

	err := c.getConnections(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "patient-uuid-123", c.patientID)
}

func TestGetConnectionsNoPatients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 0,
			"data":   []any{},
		})
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "p", "", 1)
	c.baseURL = srv.URL
	c.authToken = "test-jwt"

	err := c.getConnections(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoPatients)
}

func TestGetEntriesSuccess(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/llu/auth/login":
			step++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": map[string]any{
					"authTicket": map[string]any{
						"token":   "jwt-token",
						"expires": 9999999999,
					},
				},
			})
		case "/llu/connections":
			step++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": []map[string]any{
					{"patientId": "p-123"},
				},
			})
		case "/llu/connections/p-123/graph":
			step++
			assert.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"graphData": []map[string]any{
						{
							"FactoryTimestamp": "4/18/2026 12:30:00 PM",
							"ValueInMgPerDl":   120,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": "4/18/2026 12:35:00 PM",
							"ValueInMgPerDl":   150,
							"TrendArrow":       4,
						},
					},
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "", 42)
	c.baseURL = srv.URL

	readings, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, step) // login + connections + graph
	require.Len(t, readings, 2)

	assert.Equal(t, int64(42), readings[0].UserID)
	assert.InDelta(t, 120.0/mgdlToMmol, readings[0].ValueMmol, 0.01)
	assert.Equal(t, "librelinkup", readings[0].Source)
	require.NotNil(t, readings[0].Trend)
	assert.Equal(t, "Flat", *readings[0].Trend)

	assert.Equal(t, int64(42), readings[1].UserID)
	assert.InDelta(t, 150.0/mgdlToMmol, readings[1].ValueMmol, 0.01)
	require.NotNil(t, readings[1].Trend)
	assert.Equal(t, "FortyFiveUp", *readings[1].Trend)
}

func TestGetEntriesFiltersSince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/llu/connections/p-1/graph" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"graphData": []map[string]any{
						{
							"FactoryTimestamp": "4/17/2026 10:00:00 AM",
							"ValueInMgPerDl":   100,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": "4/18/2026 10:00:00 AM",
							"ValueInMgPerDl":   120,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": "4/19/2026 10:00:00 AM",
							"ValueInMgPerDl":   140,
							"TrendArrow":       5,
						},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "", 1)
	c.baseURL = srv.URL
	c.authToken = "jwt"
	c.patientID = "p-1"

	since := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	readings, err := c.GetEntries(context.Background(), since, 10)
	require.NoError(t, err)
	assert.Len(t, readings, 2)
	assert.InDelta(t, 120.0/mgdlToMmol, readings[0].ValueMmol, 0.01)
	assert.InDelta(t, 140.0/mgdlToMmol, readings[1].ValueMmol, 0.01)
}

func TestGetEntriesFiltersInvalidValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/llu/connections/p-1/graph" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"graphData": []map[string]any{
						{
							"FactoryTimestamp": "4/18/2026 12:00:00 PM",
							"ValueInMgPerDl":   120,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": "4/18/2026 12:05:00 PM",
							"ValueInMgPerDl":   0,
							"TrendArrow":       3,
						},
						{
							"FactoryTimestamp": "4/18/2026 12:10:00 PM",
							"ValueInMgPerDl":   -5,
							"TrendArrow":       3,
						},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "", 1)
	c.baseURL = srv.URL
	c.authToken = "jwt"
	c.patientID = "p-1"

	readings, err := c.GetEntries(context.Background(), time.Time{}, 10)
	require.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.InDelta(t, 120.0/mgdlToMmol, readings[0].ValueMmol, 0.01)
}

func TestTrendArrowMapping(t *testing.T) {
	tests := []struct {
		arrow    int
		expected *string
	}{
		{1, strPtr("SingleDown")},
		{2, strPtr("FortyFiveDown")},
		{3, strPtr("Flat")},
		{4, strPtr("FortyFiveUp")},
		{5, strPtr("SingleUp")},
		{0, nil},
		{6, nil},
		{-1, nil},
		{99, nil},
	}

	for _, tt := range tests {
		result := mapTrendArrow(tt.arrow)
		if tt.expected == nil {
			assert.Nil(t, result, "arrow %d should map to nil", tt.arrow)
		} else {
			require.NotNil(t, result, "arrow %d should not be nil", tt.arrow)
			assert.Equal(t, *tt.expected, *result, "arrow %d", tt.arrow)
		}
	}
}

func TestVerifyConnectionSuccess(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/llu/auth/login":
			step++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": map[string]any{
					"authTicket": map[string]any{
						"token":   "jwt-token",
						"expires": 9999999999,
					},
				},
			})
		case "/llu/connections":
			step++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": []map[string]any{
					{"patientId": "p-abc"},
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "", 42)
	c.baseURL = srv.URL

	err := c.VerifyConnection(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, step) // login + connections
	assert.Equal(t, "jwt-token", c.authToken)
	assert.Equal(t, "p-abc", c.patientID)
}

func TestGetEntriesLimitsCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/llu/connections/p-1/graph" {
			entries := make([]map[string]any, 10)
			for i := range 10 {
				entries[i] = map[string]any{
					"FactoryTimestamp": "4/18/2026 12:00:00 PM",
					"ValueInMgPerDl":   100 + i*10,
					"TrendArrow":       3,
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"graphData": entries,
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "", 1)
	c.baseURL = srv.URL
	c.authToken = "jwt"
	c.patientID = "p-1"

	readings, err := c.GetEntries(context.Background(), time.Time{}, 3)
	require.NoError(t, err)
	assert.Len(t, readings, 3)
}

func TestDetectedRegion(t *testing.T) {
	c := NewClient("u@e.com", "pass", "", 1)
	assert.Equal(t, "", c.DetectedRegion())

	c.region = "de"
	assert.Equal(t, "de", c.DetectedRegion())
}

func TestNewClientWithRegion(t *testing.T) {
	c := NewClient("u@e.com", "pass", "eu", 1)
	assert.Equal(t, "eu", c.region)
	assert.Equal(t, "https://api-eu.libreview.io", c.baseURL)
}

func TestNewClientWithoutRegion(t *testing.T) {
	c := NewClient("u@e.com", "pass", "", 1)
	assert.Equal(t, "", c.region)
	assert.Equal(t, "https://api.libreview.io", c.baseURL)
}

func TestNewClientRURegion(t *testing.T) {
	c := NewClient("u@e.com", "pass", "ru", 1)
	assert.Equal(t, "ru", c.region)
	assert.Equal(t, "https://api.libreview.ru", c.baseURL)
}

func TestRURegionUsesLLUPrefix(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		// All RU requests must carry the same iOS-app headers.
		assert.Equal(t, "llu.ios", r.Header.Get("product"))
		assert.Equal(t, appVersion, r.Header.Get("version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/llu/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data": map[string]any{
					"authTicket": map[string]any{"token": "jwt", "expires": 9999999999},
					"user":       map[string]any{"id": "u-1"},
				},
			})
		case "/llu/connections":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 0,
				"data":   []map[string]any{{"patientId": "p-ru"}},
			})
		}
	}))
	defer srv.Close()

	c := NewClient("u@e.com", "pass", "ru", 42)
	c.baseURL = srv.URL

	err := c.VerifyConnection(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"/llu/auth/login", "/llu/connections"}, paths)
}

func strPtr(s string) *string { return &s }
