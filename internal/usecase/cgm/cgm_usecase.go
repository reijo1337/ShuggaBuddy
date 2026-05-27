package cgm

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/pkg/librelinkup"
	"github.com/gmtantsevov/shuggabuddy/pkg/nightscout"
)

type TokenEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type UseCase struct {
	cgmRepo     domain.CGMConnectionRepository
	glucoseRepo domain.GlucoseRepository
	encryptor   TokenEncryptor
	lluBaseURL  string
}

// Option configures a UseCase.
type Option func(*UseCase)

// WithLibreLinkUpBaseURL overrides the LibreLinkUp API base URL.
// Intended for tests that point the client at a mock server.
func WithLibreLinkUpBaseURL(baseURL string) Option {
	return func(uc *UseCase) { uc.lluBaseURL = baseURL }
}

func New(cgmRepo domain.CGMConnectionRepository, glucoseRepo domain.GlucoseRepository, encryptor TokenEncryptor, opts ...Option) *UseCase {
	uc := &UseCase{
		cgmRepo:     cgmRepo,
		glucoseRepo: glucoseRepo,
		encryptor:   encryptor,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func (uc *UseCase) libreLinkUpOpts() []librelinkup.Option {
	if uc.lluBaseURL == "" {
		return nil
	}
	return []librelinkup.Option{librelinkup.WithBaseURL(uc.lluBaseURL)}
}

func (uc *UseCase) buildClient(conn *domain.CGMConnection, decryptedToken string) domain.CGMClient {
	switch conn.Provider {
	case domain.CGMProviderNightscout:
		return nightscout.NewClient(conn.BaseURL, decryptedToken, conn.UserID)
	case domain.CGMProviderLibreLinkUp:
		region := ""
		if conn.Region != nil {
			region = *conn.Region
		}
		return librelinkup.NewClient(conn.BaseURL, decryptedToken, region, conn.UserID, uc.libreLinkUpOpts()...)
	default:
		return nil
	}
}

func (uc *UseCase) AddConnection(ctx context.Context, userID int64, provider domain.CGMProvider, credential1, credential2 string) error {
	var conn *domain.CGMConnection

	switch provider {
	case domain.CGMProviderNightscout:
		if err := validateURL(credential1); err != nil {
			return err
		}

		client := nightscout.NewClient(credential1, credential2, userID)
		if err := client.VerifyConnection(ctx); err != nil {
			return err
		}

		encrypted, err := uc.encryptor.Encrypt(credential2)
		if err != nil {
			return fmt.Errorf("cgm.AddConnection: encrypt: %w", err)
		}

		conn = &domain.CGMConnection{
			UserID:   userID,
			Provider: domain.CGMProviderNightscout,
			BaseURL:  credential1,
			APIToken: encrypted,
			Active:   true,
		}

	case domain.CGMProviderLibreLinkUp:
		if err := validateEmail(credential1); err != nil {
			return err
		}

		client := librelinkup.NewClient(credential1, credential2, "", userID, uc.libreLinkUpOpts()...)
		if err := client.VerifyConnection(ctx); err != nil {
			return err
		}

		encrypted, err := uc.encryptor.Encrypt(credential2)
		if err != nil {
			return fmt.Errorf("cgm.AddConnection: encrypt: %w", err)
		}

		region := client.DetectedRegion()
		var regionPtr *string
		if region != "" {
			regionPtr = &region
		}

		conn = &domain.CGMConnection{
			UserID:   userID,
			Provider: domain.CGMProviderLibreLinkUp,
			BaseURL:  credential1,
			APIToken: encrypted,
			Region:   regionPtr,
			Active:   true,
		}

	default:
		return fmt.Errorf("cgm.AddConnection: unsupported provider %s", provider)
	}

	if err := uc.cgmRepo.Upsert(ctx, conn); err != nil {
		return fmt.Errorf("cgm.AddConnection: %w", err)
	}

	return nil
}

func (uc *UseCase) TestConnection(ctx context.Context, userID int64) error {
	conn, err := uc.cgmRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("cgm.TestConnection: %w", err)
	}
	if conn == nil {
		return fmt.Errorf("cgm.TestConnection: no connection found")
	}

	token, err := uc.encryptor.Decrypt(conn.APIToken)
	if err != nil {
		return fmt.Errorf("cgm.TestConnection: decrypt: %w", err)
	}

	client := uc.buildClient(conn, token)
	if client == nil {
		return fmt.Errorf("cgm.TestConnection: unknown provider %s", conn.Provider)
	}
	return client.VerifyConnection(ctx)
}

func (uc *UseCase) RemoveConnection(ctx context.Context, userID int64) error {
	return uc.cgmRepo.Delete(ctx, userID)
}

func (uc *UseCase) GetConnection(ctx context.Context, userID int64) (*domain.CGMConnection, error) {
	conn, err := uc.cgmRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("cgm.GetConnection: %w", err)
	}
	if conn == nil {
		return nil, nil
	}

	token, err := uc.encryptor.Decrypt(conn.APIToken)
	if err != nil {
		conn.APIToken = "****"
	} else {
		conn.APIToken = redactToken(token)
	}

	return conn, nil
}

func (uc *UseCase) SyncUser(ctx context.Context, conn *domain.CGMConnection) error {
	token, err := uc.encryptor.Decrypt(conn.APIToken)
	if err != nil {
		return fmt.Errorf("cgm.SyncUser: decrypt: %w", err)
	}

	client := uc.buildClient(conn, token)
	if client == nil {
		return fmt.Errorf("cgm.SyncUser: unknown provider %s", conn.Provider)
	}

	since := time.Now().UTC().Add(-24 * time.Hour)
	if conn.LastSyncedAt != nil {
		since = conn.LastSyncedAt.Add(-5 * time.Minute)
	}

	readings, err := client.GetEntries(ctx, since, 288)
	if err != nil {
		return fmt.Errorf("cgm.SyncUser: %w", err)
	}

	if len(readings) == 0 {
		return nil
	}

	var latest time.Time
	for _, r := range readings {
		if r.RecordedAt.After(latest) {
			latest = r.RecordedAt
		}
	}

	if _, err := uc.glucoseRepo.SaveBatch(ctx, readings); err != nil {
		return fmt.Errorf("cgm.SyncUser: save: %w", err)
	}

	if err := uc.cgmRepo.UpdateLastSyncedAt(ctx, conn.ID, latest); err != nil {
		return fmt.Errorf("cgm.SyncUser: update sync time: %w", err)
	}

	return nil
}

func validateURL(rawURL string) error {
	if len(rawURL) > 500 {
		return fmt.Errorf("cgm: URL too long (max 500 characters)")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("cgm: invalid URL: %w", err)
	}

	if u.User != nil {
		return fmt.Errorf("cgm: URL must not contain credentials")
	}

	host := u.Hostname()
	isLocal := host == "localhost" || host == "127.0.0.1" || host == "::1"

	if u.Scheme != "https" && !isLocal {
		return fmt.Errorf("cgm: HTTPS is required (got %s)", u.Scheme)
	}

	return nil
}

func validateEmail(email string) error {
	if len(email) > 254 {
		return fmt.Errorf("cgm: email too long")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return fmt.Errorf("cgm: invalid email format")
	}
	return nil
}

func redactToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}
