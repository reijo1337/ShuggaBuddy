package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/user"
)

func TestGetOrCreateUser_NewUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(nil, nil)

	userRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *domain.User) error {
			u.ID = 1
			return nil
		})

	extAccRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *domain.ExternalAccount) error {
			assert.Equal(t, int64(1), acc.UserID)
			assert.Equal(t, domain.ProviderTelegram, acc.Provider)
			assert.Equal(t, "123", acc.ExternalID)
			assert.Equal(t, "Test", acc.DisplayName)
			return nil
		})

	uc := user.New(userRepo, extAccRepo)

	u, acc, created, err := uc.GetOrCreateUser(context.Background(), domain.ProviderTelegram, "123", "Test")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, int64(1), u.ID)
	assert.Equal(t, domain.UnitsMmol, u.Units)
	assert.Equal(t, "Test", acc.DisplayName)
}

func TestGetOrCreateUser_ExistingUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	existingAcc := &domain.ExternalAccount{
		ID:          1,
		UserID:      42,
		Provider:    domain.ProviderTelegram,
		ExternalID:  "123",
		DisplayName: "Test",
	}
	existingUser := &domain.User{
		ID:    42,
		Units: domain.UnitsMgdl,
	}

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(existingAcc, nil)

	userRepo.EXPECT().
		GetByID(gomock.Any(), int64(42)).
		Return(existingUser, nil)

	uc := user.New(userRepo, extAccRepo)

	u, acc, created, err := uc.GetOrCreateUser(context.Background(), domain.ProviderTelegram, "123", "Test")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, domain.UnitsMgdl, u.Units)
	assert.Equal(t, "Test", acc.DisplayName)
}

func TestGetOrCreateUser_GetByProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(nil, errors.New("db error"))

	uc := user.New(userRepo, extAccRepo)

	_, _, _, err := uc.GetOrCreateUser(context.Background(), domain.ProviderTelegram, "123", "Test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestGetOrCreateUser_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(nil, nil)

	userRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("insert failed"))

	uc := user.New(userRepo, extAccRepo)

	_, _, _, err := uc.GetOrCreateUser(context.Background(), domain.ProviderTelegram, "123", "Test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
}

func TestGetOrCreateUser_CreateExtAccError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(nil, nil)

	userRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u *domain.User) error {
			u.ID = 1
			return nil
		})

	extAccRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(errors.New("ext acc failed"))

	uc := user.New(userRepo, extAccRepo)

	_, _, _, err := uc.GetOrCreateUser(context.Background(), domain.ProviderTelegram, "123", "Test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ext acc failed")
}

func TestGetProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	expectedAcc := &domain.ExternalAccount{
		UserID:      42,
		Provider:    domain.ProviderTelegram,
		ExternalID:  "123",
		DisplayName: "Alice",
	}
	expectedUser := &domain.User{ID: 42, Units: domain.UnitsMmol}

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(expectedAcc, nil)

	userRepo.EXPECT().
		GetByID(gomock.Any(), int64(42)).
		Return(expectedUser, nil)

	uc := user.New(userRepo, extAccRepo)

	u, acc, err := uc.GetProfile(context.Background(), domain.ProviderTelegram, "123")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "Alice", acc.DisplayName)
}

func TestGetProfile_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "999").
		Return(nil, nil)

	uc := user.New(userRepo, extAccRepo)

	u, acc, err := uc.GetProfile(context.Background(), domain.ProviderTelegram, "999")
	require.NoError(t, err)
	assert.Nil(t, u)
	assert.Nil(t, acc)
}

func TestGetProfile_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	extAccRepo.EXPECT().
		GetByProvider(gomock.Any(), domain.ProviderTelegram, "123").
		Return(nil, errors.New("db error"))

	uc := user.New(userRepo, extAccRepo)

	_, _, err := uc.GetProfile(context.Background(), domain.ProviderTelegram, "123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestUpdateUnits_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	userRepo.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMgdl).
		Return(nil)

	uc := user.New(userRepo, extAccRepo)

	err := uc.UpdateUnits(context.Background(), 1, domain.UnitsMgdl)
	require.NoError(t, err)
}

func TestUpdateUnits_InvalidUnits(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	uc := user.New(userRepo, extAccRepo)

	err := uc.UpdateUnits(context.Background(), 1, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid units")
}

func TestUpdateUnits_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	userRepo.EXPECT().
		UpdateUnits(gomock.Any(), int64(1), domain.UnitsMmol).
		Return(errors.New("db error"))

	uc := user.New(userRepo, extAccRepo)

	err := uc.UpdateUnits(context.Background(), 1, domain.UnitsMmol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestUpdateCarbsPerUnit_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	userRepo.EXPECT().
		UpdateCarbsPerUnit(gomock.Any(), int64(1), 10.0).
		Return(nil)

	uc := user.New(userRepo, extAccRepo)
	err := uc.UpdateCarbsPerUnit(context.Background(), 1, 10.0)
	require.NoError(t, err)
}

func TestUpdateCarbsPerUnit_TooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	uc := user.New(userRepo, extAccRepo)
	err := uc.UpdateCarbsPerUnit(context.Background(), 1, 0.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestUpdateCarbsPerUnit_TooHigh(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	uc := user.New(userRepo, extAccRepo)
	err := uc.UpdateCarbsPerUnit(context.Background(), 1, 51.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestUpdateCarbsPerUnit_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

	userRepo.EXPECT().
		UpdateCarbsPerUnit(gomock.Any(), int64(1), 12.0).
		Return(errors.New("db error"))

	uc := user.New(userRepo, extAccRepo)
	err := uc.UpdateCarbsPerUnit(context.Background(), 1, 12.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestUpdateSettings(t *testing.T) {
	type args struct {
		userID    int64
		targetMin float64
		targetMax float64
		basalDrug string
		basalTime string
	}

	tests := []struct {
		name        string
		args        args
		setupMock   func(userRepo *mocks.MockUserRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "valid input",
			args: args{userID: 1, targetMin: 3.9, targetMax: 10.0, basalDrug: "Lantus", basalTime: "22:00"},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					UpdateSettings(gomock.Any(), int64(1), 3.9, 10.0, "Lantus", "22:00").
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "valid: empty basalDrug and basalTime",
			args: args{userID: 2, targetMin: 4.0, targetMax: 8.0, basalDrug: "", basalTime: ""},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					UpdateSettings(gomock.Any(), int64(2), 4.0, 8.0, "", "").
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:      "invalid: targetMin < 1.0",
			args:      args{userID: 1, targetMin: 0.5, targetMax: 10.0, basalDrug: "", basalTime: ""},
			setupMock: func(_ *mocks.MockUserRepository) {},
			wantErr:   true,
		},
		{
			name:      "invalid: targetMax > 33.3",
			args:      args{userID: 1, targetMin: 3.9, targetMax: 34.0, basalDrug: "", basalTime: ""},
			setupMock: func(_ *mocks.MockUserRepository) {},
			wantErr:   true,
		},
		{
			name:      "invalid: targetMax <= targetMin",
			args:      args{userID: 1, targetMin: 10.0, targetMax: 5.0, basalDrug: "", basalTime: ""},
			setupMock: func(_ *mocks.MockUserRepository) {},
			wantErr:   true,
		},
		{
			name:      "invalid: targetMax equals targetMin",
			args:      args{userID: 1, targetMin: 7.0, targetMax: 7.0, basalDrug: "", basalTime: ""},
			setupMock: func(_ *mocks.MockUserRepository) {},
			wantErr:   true,
		},
		{
			name:        "invalid basalTime: bad hours 25:00",
			args:        args{userID: 1, targetMin: 3.9, targetMax: 10.0, basalDrug: "", basalTime: "25:00"},
			setupMock:   func(_ *mocks.MockUserRepository) {},
			wantErr:     true,
			errContains: "basalTime",
		},
		{
			name:        "invalid basalTime: bad minutes 10:60",
			args:        args{userID: 1, targetMin: 3.9, targetMax: 10.0, basalDrug: "", basalTime: "10:60"},
			setupMock:   func(_ *mocks.MockUserRepository) {},
			wantErr:     true,
			errContains: "basalTime",
		},
		{
			name:        "invalid basalTime: non-numeric abc",
			args:        args{userID: 1, targetMin: 3.9, targetMax: 10.0, basalDrug: "", basalTime: "abc"},
			setupMock:   func(_ *mocks.MockUserRepository) {},
			wantErr:     true,
			errContains: "basalTime",
		},
		{
			name: "repo returns error",
			args: args{userID: 1, targetMin: 3.9, targetMax: 10.0, basalDrug: "Tresiba", basalTime: "08:00"},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					UpdateSettings(gomock.Any(), int64(1), 3.9, 10.0, "Tresiba", "08:00").
					Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			userRepo := mocks.NewMockUserRepository(ctrl)
			extAccRepo := mocks.NewMockExternalAccountRepository(ctrl)

			tt.setupMock(userRepo)

			uc := user.New(userRepo, extAccRepo)
			err := uc.UpdateSettings(context.Background(), tt.args.userID, tt.args.targetMin, tt.args.targetMax, tt.args.basalDrug, tt.args.basalTime)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
