package librelinkup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	defaultBaseURL = "https://api.libreview.io"
	baseURLFormat  = "https://api-%s.libreview.io"

	pathPrefix = "/llu"

	appVersion = "4.18.0"
	userAgent  = "Mozilla/5.0 (iPhone; CPU OS 26_3.1 like Mac OS X) AppleWebKit/536.26 (KHTML, like Gecko) Version/26.3.1 Mobile/10A5355d Safari/8536.25"

	mgdlToMmol = 18.0182

	timestampLayout = "1/2/2006 3:04:05 PM"
)

// Regional base URLs that don't follow the standard pattern.
// All regions share the same /llu path prefix and header scheme — only the host differs.
var regionalBaseURLs = map[string]string{
	"ru": "https://api.libreview.ru",
}

var (
	ErrUnauthorized   = errors.New("librelinkup: unauthorized")
	ErrNoPatients     = errors.New("librelinkup: no patients found")
	ErrActionRequired = errors.New("librelinkup: additional action required in LibreLinkUp app")
	ErrAPI            = errors.New("librelinkup: api error")
)

type Client struct {
	email           string
	password        string
	region          string
	userID          int64
	authToken       string
	accountID       string
	patientID       string
	baseURL         string
	baseURLOverride string
	httpClient      *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL forces the client to use the given base URL regardless of region.
// Intended for tests that point the client at a mock server.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURLOverride = url }
}

func NewClient(email, password, region string, userID int64, opts ...Option) *Client {
	jar, _ := cookiejar.New(nil)

	// Reorder TLS cipher suites to avoid bot detection by JA3 fingerprint.
	defaultSuites := tls.CipherSuites()
	cipherIDs := make([]uint16, len(defaultSuites))
	for i, s := range defaultSuites {
		cipherIDs[i] = s.ID
	}
	if len(cipherIDs) > 2 {
		cipherIDs[1], cipherIDs[2] = cipherIDs[2], cipherIDs[1]
	}

	c := &Client{
		email:    email,
		password: password,
		region:   region,
		userID:   userID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					CipherSuites: cipherIDs,
				},
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.baseURL = c.resolveBaseURL(region)
	return c
}

func (c *Client) resolveBaseURL(region string) string {
	if c.baseURLOverride != "" {
		return c.baseURLOverride
	}
	if region == "" {
		return defaultBaseURL
	}
	if url, ok := regionalBaseURLs[strings.ToLower(region)]; ok {
		return url
	}
	return fmt.Sprintf(baseURLFormat, region)
}

func (c *Client) DetectedRegion() string {
	return c.region
}

// VerifyConnection implements domain.CGMClient.
func (c *Client) VerifyConnection(ctx context.Context) error {
	if err := c.login(ctx); err != nil {
		// If no region was set and login failed, try RU endpoint as fallback.
		if c.region == "" && errors.Is(err, ErrUnauthorized) {
			c.region = "ru"
			c.baseURL = c.resolveBaseURL(c.region)
			if ruErr := c.login(ctx); ruErr != nil {
				return err // return original error
			}
			return c.getConnections(ctx)
		}
		return err
	}
	return c.getConnections(ctx)
}

// GetEntries implements domain.CGMClient.
func (c *Client) GetEntries(ctx context.Context, since time.Time, count int) ([]domain.GlucoseReading, error) {
	if c.authToken == "" {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}
	if c.patientID == "" {
		if err := c.getConnections(ctx); err != nil {
			return nil, err
		}
	}

	entries, err := c.getGraph(ctx)
	if err != nil {
		return nil, err
	}

	readings := make([]domain.GlucoseReading, 0, len(entries))
	for _, e := range entries {
		if e.ValueInMgPerDl <= 0 {
			continue
		}

		t, err := time.Parse(timestampLayout, e.FactoryTimestamp)
		if err != nil {
			continue
		}

		if !since.IsZero() && t.Before(since) {
			continue
		}

		trend := mapTrendArrow(e.TrendArrow)
		readings = append(readings, domain.GlucoseReading{
			UserID:     c.userID,
			ValueMmol:  float64(e.ValueInMgPerDl) / mgdlToMmol,
			Source:     "librelinkup",
			Trend:      trend,
			RecordedAt: t,
		})

		if len(readings) >= count {
			break
		}
	}

	return readings, nil
}

// login handles the full login flow including redirect.
func (c *Client) login(ctx context.Context) error {
	body := map[string]string{
		"email":    c.email,
		"password": c.password,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("librelinkup.login: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+pathPrefix+"/auth/login", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("librelinkup.login: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("librelinkup.login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d, body: %s", ErrAPI, resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("librelinkup.login: read body: %w", err)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return fmt.Errorf("librelinkup.login: decode: %w", err)
	}

	if loginResp.Status == 2 {
		return fmt.Errorf("%w: %s", ErrUnauthorized, string(respBody))
	}

	if loginResp.Status == 4 {
		return ErrActionRequired
	}

	// Handle redirect to regional endpoint.
	if loginResp.Data.Redirect && loginResp.Data.Region != "" {
		c.region = loginResp.Data.Region
		c.baseURL = c.resolveBaseURL(c.region)

		// For non-special regions, try the country config endpoint for the exact URL.
		// RU has a hardcoded host, other regions may use region-specific shards.
		if _, special := regionalBaseURLs[strings.ToLower(c.region)]; !special {
			if regionURL, err := c.resolveRegionURL(ctx, c.region); err == nil {
				c.baseURL = regionURL
			}
		}
		return c.login(ctx)
	}

	if loginResp.Status != 0 {
		return fmt.Errorf("%w: unexpected status %d", ErrAPI, loginResp.Status)
	}

	if loginResp.Data.AuthTicket.Token == "" {
		return ErrUnauthorized
	}

	c.authToken = loginResp.Data.AuthTicket.Token
	c.accountID = loginResp.Data.User.ID
	return nil
}

// resolveRegionURL fetches the regional API URL from the country config.
func (c *Client) resolveRegionURL(ctx context.Context, region string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathPrefix+"/config/country?country=DE", http.NoBody)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("country config: status %d", resp.StatusCode)
	}

	var countryResp countryResponse
	if err := json.NewDecoder(resp.Body).Decode(&countryResp); err != nil {
		return "", err
	}

	node, ok := countryResp.Data.RegionalMap[region]
	if !ok {
		return "", fmt.Errorf("region %q not found in regional map", region)
	}

	return node.LslAPI, nil
}

// getConnections fetches the patient ID from connections.
func (c *Client) getConnections(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathPrefix+"/connections", http.NoBody)
	if err != nil {
		return fmt.Errorf("librelinkup.getConnections: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("librelinkup.getConnections: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrAPI, resp.StatusCode)
	}

	var connResp connectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&connResp); err != nil {
		return fmt.Errorf("librelinkup.getConnections: decode: %w", err)
	}

	if len(connResp.Data) == 0 {
		return ErrNoPatients
	}

	c.patientID = connResp.Data[0].PatientID
	return nil
}

// getGraph fetches graph data for the patient.
func (c *Client) getGraph(ctx context.Context) ([]graphEntry, error) {
	url := fmt.Sprintf("%s%s/connections/%s/graph", c.baseURL, pathPrefix, c.patientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("librelinkup.getGraph: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("librelinkup.getGraph: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrAPI, resp.StatusCode)
	}

	var graphResp graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphResp); err != nil {
		return nil, fmt.Errorf("librelinkup.getGraph: decode: %w", err)
	}

	return graphResp.Data.GraphData, nil
}

// setHeaders sets common headers for LibreLinkUp API requests.
// Header scheme is identical across all regions (including RU) — only the host differs.
// See dumps from iOS LibreLinkUp RU app confirming this.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("connection", "Keep-Alive")
	req.Header.Set("accept", "*/*")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("product", "llu.ios")
	req.Header.Set("version", appVersion)

	if c.accountID != "" {
		req.Header.Set("account-id", fmt.Sprintf("%x", sha256.Sum256([]byte(c.accountID))))
	} else {
		req.Header.Set("account-id", "")
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}

func mapTrendArrow(arrow int) *string {
	var trend string
	switch arrow {
	case 1:
		trend = "SingleDown"
	case 2:
		trend = "FortyFiveDown"
	case 3:
		trend = "Flat"
	case 4:
		trend = "FortyFiveUp"
	case 5:
		trend = "SingleUp"
	default:
		return nil
	}
	return &trend
}

// API response types.

type loginResponse struct {
	Status int `json:"status"`
	Data   struct {
		Redirect   bool   `json:"redirect"`
		Region     string `json:"region"`
		AuthTicket struct {
			Token   string `json:"token"`
			Expires int64  `json:"expires"`
		} `json:"authTicket"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"data"`
}

type countryResponse struct {
	Data struct {
		RegionalMap map[string]struct {
			LslAPI string `json:"lslApi"`
		} `json:"regionalMap"`
	} `json:"data"`
}

type connectionsResponse struct {
	Status int `json:"status"`
	Data   []struct {
		PatientID string `json:"patientId"`
	} `json:"data"`
}

type graphEntry struct {
	FactoryTimestamp string `json:"FactoryTimestamp"`
	ValueInMgPerDl   int    `json:"ValueInMgPerDl"`
	TrendArrow       int    `json:"TrendArrow"`
}

type graphResponse struct {
	Data struct {
		GraphData []graphEntry `json:"graphData"`
	} `json:"data"`
}
