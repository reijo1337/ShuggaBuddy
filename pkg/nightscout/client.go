package nightscout

import (
	"context"
	"crypto/sha1" //nolint:gosec // Nightscout protocol requires SHA1-hashed api-secret
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const MgdlToMmol = 18.0182

var (
	ErrUnauthorized = errors.New("nightscout: unauthorized")
	ErrNotFound     = errors.New("nightscout: not found")
	ErrAPI          = errors.New("nightscout: api error")
)

type Entry struct {
	ID        string `json:"_id"`
	SGV       int    `json:"sgv"`
	Direction string `json:"direction"`
	DateMs    int64  `json:"date"`
	Type      string `json:"type"`
}

func (e Entry) ToGlucoseReading(userID int64) domain.GlucoseReading {
	trend := NormalizeTrend(e.Direction)
	return domain.GlucoseReading{
		UserID:     userID,
		ValueMmol:  float64(e.SGV) / MgdlToMmol,
		Source:     "nightscout",
		Trend:      trend,
		RecordedAt: time.UnixMilli(e.DateMs),
	}
}

func NormalizeTrend(direction string) *string {
	switch direction {
	case "DoubleUp", "SingleUp", "FortyFiveUp", "Flat", "FortyFiveDown", "SingleDown", "DoubleDown":
		return &direction
	default:
		return nil
	}
}

type Client struct {
	baseURL    string
	apiSecret  string
	userID     int64
	httpClient *http.Client
}

func NewClient(baseURL, apiSecret string, userID int64) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiSecret: apiSecret,
		userID:    userID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) isToken() bool {
	return strings.Contains(c.apiSecret, "-")
}

func (c *Client) GetEntries(ctx context.Context, since time.Time, count int) ([]domain.GlucoseReading, error) {
	url := fmt.Sprintf("%s/api/v1/entries.json?find[type]=sgv&count=%d", c.baseURL, count)

	if !since.IsZero() {
		url += "&find[dateString][$gte]=" + since.UTC().Format(time.RFC3339)
	}

	if c.isToken() {
		url += "&token=" + c.apiSecret
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("nightscout.GetEntries: %w", err)
	}

	if !c.isToken() {
		hash := sha1.Sum([]byte(c.apiSecret)) //nolint:gosec // Nightscout protocol requires SHA1
		req.Header.Set("api-secret", hex.EncodeToString(hash[:]))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nightscout.GetEntries: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp.StatusCode); err != nil {
		return nil, err
	}

	var entries []Entry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("nightscout.GetEntries: decode: %w", err)
	}

	readings := make([]domain.GlucoseReading, 0, len(entries))
	for _, e := range entries {
		if e.SGV <= 0 {
			continue
		}
		readings = append(readings, e.ToGlucoseReading(c.userID))
	}

	return readings, nil
}

func (c *Client) VerifyConnection(ctx context.Context) error {
	url := c.baseURL + "/api/v1/status.json"

	if c.isToken() {
		url += "?token=" + c.apiSecret
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("nightscout.VerifyConnection: %w", err)
	}

	if !c.isToken() {
		hash := sha1.Sum([]byte(c.apiSecret)) //nolint:gosec // Nightscout protocol requires SHA1
		req.Header.Set("api-secret", hex.EncodeToString(hash[:]))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nightscout.VerifyConnection: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return checkStatus(resp.StatusCode)
}

func checkStatus(code int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return ErrUnauthorized
	case code == http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("%w: status %d", ErrAPI, code)
	}
}
