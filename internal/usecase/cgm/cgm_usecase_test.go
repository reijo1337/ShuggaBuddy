package cgm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/pkg/nightscout"
)

type stubEncryptor struct{}

func (s *stubEncryptor) Encrypt(plaintext string) (string, error) { return "enc:" + plaintext, nil }
func (s *stubEncryptor) Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) > 4 && ciphertext[:4] == "enc:" {
		return ciphertext[4:], nil
	}
	return ciphertext, nil
}

func TestAddConnectionNightscoutRejectsHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, domain.CGMProviderNightscout, "http://example.com", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestAddConnectionNightscoutAllowsLocalhost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	cgmRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, domain.CGMProviderNightscout, srv.URL, "secret")
	assert.NoError(t, err)
}

func TestAddConnectionNightscoutVerifiesFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, domain.CGMProviderNightscout, srv.URL, "bad")
	require.Error(t, err)
	assert.ErrorIs(t, err, nightscout.ErrUnauthorized)
}

func TestAddConnectionLibreLinkUpValidatesEmailBeforeConnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	// Valid email passes validation but fails at VerifyConnection (no real server).
	// This confirms the flow reaches the client call.
	err := uc.AddConnection(context.Background(), 1, domain.CGMProviderLibreLinkUp, "test@example.com", "password")
	require.Error(t, err)
	// Should NOT be an email validation error
	assert.NotContains(t, err.Error(), "invalid email")
}

func TestAddConnectionLibreLinkUpRejectsInvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, domain.CGMProviderLibreLinkUp, "notanemail", "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email")
}

func TestAddConnectionUnsupportedProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, "unknown", "cred1", "cred2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestSyncUserFetchesAndSaves(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Nightscout GetEntries now returns []domain.GlucoseReading directly
	readings := []domain.GlucoseReading{
		{UserID: 42, ValueMmol: 6.66, Source: "nightscout", RecordedAt: now.Add(-5 * time.Minute)},
		{UserID: 42, ValueMmol: 7.22, Source: "nightscout", RecordedAt: now},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/status.json" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		// The nightscout client still parses Entry JSON internally,
		// so we return entries in the nightscout format.
		entries := []map[string]any{
			{"sgv": 120, "direction": "Flat", "date": now.Add(-5 * time.Minute).UnixMilli()},
			{"sgv": 130, "direction": "SingleUp", "date": now.UnixMilli()},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	glucoseRepo.EXPECT().SaveBatch(gomock.Any(), gomock.Len(2)).Return(2, nil)
	cgmRepo.EXPECT().UpdateLastSyncedAt(gomock.Any(), int64(1), gomock.Any()).Return(nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	conn := &domain.CGMConnection{
		ID:       1,
		UserID:   42,
		Provider: domain.CGMProviderNightscout,
		BaseURL:  srv.URL,
		APIToken: "enc:testsecret",
	}

	err := uc.SyncUser(context.Background(), conn)
	assert.NoError(t, err)
	_ = readings // readings shown for documentation; actual data comes from nightscout client
}

func TestSyncUserHandlesEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	conn := &domain.CGMConnection{
		ID:       1,
		UserID:   42,
		Provider: domain.CGMProviderNightscout,
		BaseURL:  srv.URL,
		APIToken: "enc:secret",
	}

	err := uc.SyncUser(context.Background(), conn)
	assert.NoError(t, err)
}

func TestSyncUserUnknownProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	conn := &domain.CGMConnection{
		ID:       1,
		UserID:   42,
		Provider: "unknown",
		BaseURL:  "https://example.com",
		APIToken: "enc:secret",
	}

	err := uc.SyncUser(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestGetConnectionRedactsToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	cgmRepo.EXPECT().GetByUserID(gomock.Any(), int64(1)).Return(&domain.CGMConnection{
		ID: 1, UserID: 1, APIToken: "enc:mysupersecrettoken", Active: true,
	}, nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	conn, err := uc.GetConnection(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.Equal(t, "****oken", conn.APIToken)
}

func TestRemoveConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	cgmRepo.EXPECT().Delete(gomock.Any(), int64(1)).Return(nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.RemoveConnection(context.Background(), 1)
	assert.NoError(t, err)
}

func TestBuildClientNightscout(t *testing.T) {
	uc := &UseCase{}
	conn := &domain.CGMConnection{
		Provider: domain.CGMProviderNightscout,
		BaseURL:  "https://my.nightscout.com",
		UserID:   1,
	}
	client := uc.buildClient(conn, "secret")
	require.NotNil(t, client)
}

func TestBuildClientLibreLinkUp(t *testing.T) {
	uc := &UseCase{}
	region := "eu"
	conn := &domain.CGMConnection{
		Provider: domain.CGMProviderLibreLinkUp,
		BaseURL:  "user@example.com",
		Region:   &region,
		UserID:   1,
	}
	client := uc.buildClient(conn, "password")
	require.NotNil(t, client)
}

func TestBuildClientLibreLinkUpNoRegion(t *testing.T) {
	uc := &UseCase{}
	conn := &domain.CGMConnection{
		Provider: domain.CGMProviderLibreLinkUp,
		BaseURL:  "user@example.com",
		UserID:   1,
	}
	client := uc.buildClient(conn, "password")
	require.NotNil(t, client)
}

func TestBuildClientUnknownProvider(t *testing.T) {
	uc := &UseCase{}
	conn := &domain.CGMConnection{
		Provider: "unknown",
	}
	client := uc.buildClient(conn, "token")
	assert.Nil(t, client)
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid", "test@example.com", false},
		{"no at", "testexample.com", true},
		{"no dot", "test@example", true},
		{"empty", "", true},
		{"too long", string(make([]byte, 255)) + "@a.b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTestConnectionUsesCorrectProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	cgmRepo.EXPECT().GetByUserID(gomock.Any(), int64(1)).Return(&domain.CGMConnection{
		ID:       1,
		UserID:   1,
		Provider: domain.CGMProviderNightscout,
		BaseURL:  srv.URL,
		APIToken: "enc:secret",
		Active:   true,
	}, nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.TestConnection(context.Background(), 1)
	assert.NoError(t, err)
}

func TestTestConnectionUnknownProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	cgmRepo.EXPECT().GetByUserID(gomock.Any(), int64(1)).Return(&domain.CGMConnection{
		ID:       1,
		UserID:   1,
		Provider: "unknown",
		BaseURL:  "https://example.com",
		APIToken: "enc:secret",
		Active:   true,
	}, nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.TestConnection(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}
