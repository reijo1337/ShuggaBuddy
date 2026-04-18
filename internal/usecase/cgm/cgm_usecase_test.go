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

func TestAddConnectionRejectsHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, "http://example.com", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestAddConnectionAllowsLocalhost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	cgmRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, srv.URL, "secret")
	assert.NoError(t, err)
}

func TestAddConnectionVerifiesFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	err := uc.AddConnection(context.Background(), 1, srv.URL, "bad")
	require.Error(t, err)
	assert.ErrorIs(t, err, nightscout.ErrUnauthorized)
}

func TestSyncUserFetchesAndSaves(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []nightscout.Entry{
		{SGV: 120, Direction: "Flat", DateMs: now.Add(-5 * time.Minute).UnixMilli()},
		{SGV: 130, Direction: "SingleUp", DateMs: now.UnixMilli()},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/status.json" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
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
}

func TestSyncUserSkipsZeroSGV(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	entries := []nightscout.Entry{
		{SGV: 0, Direction: "Flat", DateMs: now.UnixMilli()},
		{SGV: 120, Direction: "Flat", DateMs: now.Add(-time.Minute).UnixMilli()},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)

	glucoseRepo.EXPECT().SaveBatch(gomock.Any(), gomock.Len(1)).Return(1, nil)
	cgmRepo.EXPECT().UpdateLastSyncedAt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	uc := New(cgmRepo, glucoseRepo, &stubEncryptor{})

	conn := &domain.CGMConnection{
		ID: 1, UserID: 42, BaseURL: srv.URL, APIToken: "enc:secret",
	}

	err := uc.SyncUser(context.Background(), conn)
	assert.NoError(t, err)
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
		ID: 1, UserID: 42, BaseURL: srv.URL, APIToken: "enc:secret",
	}

	err := uc.SyncUser(context.Background(), conn)
	assert.NoError(t, err)
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
