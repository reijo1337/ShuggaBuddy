package cgm

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
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
}

func New(cgmRepo domain.CGMConnectionRepository, glucoseRepo domain.GlucoseRepository, encryptor TokenEncryptor) *UseCase {
	return &UseCase{
		cgmRepo:     cgmRepo,
		glucoseRepo: glucoseRepo,
		encryptor:   encryptor,
	}
}

func (uc *UseCase) AddConnection(ctx context.Context, userID int64, baseURL, apiToken string) error {
	if err := validateURL(baseURL); err != nil {
		return err
	}

	client := nightscout.NewClient(baseURL, apiToken)
	if err := client.VerifyConnection(ctx); err != nil {
		return err
	}

	encrypted, err := uc.encryptor.Encrypt(apiToken)
	if err != nil {
		return fmt.Errorf("cgm.AddConnection: encrypt: %w", err)
	}

	conn := &domain.CGMConnection{
		UserID:   userID,
		Provider: domain.CGMProviderNightscout,
		BaseURL:  baseURL,
		APIToken: encrypted,
		Active:   true,
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

	client := nightscout.NewClient(conn.BaseURL, token)
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

	client := nightscout.NewClient(conn.BaseURL, token)

	since := time.Now().UTC().Add(-24 * time.Hour)
	if conn.LastSyncedAt != nil {
		since = conn.LastSyncedAt.Add(-5 * time.Minute)
	}

	entries, err := client.GetEntries(ctx, since, 288)
	if err != nil {
		return fmt.Errorf("cgm.SyncUser: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	var readings []domain.GlucoseReading
	var latest time.Time
	for _, e := range entries {
		if e.SGV <= 0 {
			continue
		}
		r := e.ToGlucoseReading(conn.UserID)
		readings = append(readings, r)
		if r.RecordedAt.After(latest) {
			latest = r.RecordedAt
		}
	}

	if len(readings) == 0 {
		return nil
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

func redactToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}
