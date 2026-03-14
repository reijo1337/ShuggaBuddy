package insulin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/insulin"
)

func TestSaveDose_Valid_Bolus(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, d *domain.InsulinDose) error {
			assert.Equal(t, int64(1), d.UserID)
			assert.InDelta(t, 8.0, d.DoseUnits, 0.001)
			assert.Equal(t, domain.InsulinTypeBolus, d.InsulinType)
			assert.Equal(t, "Новорапид", d.Drug)
			return nil
		})

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 8.0, domain.InsulinTypeBolus, "Новорапид")
	require.NoError(t, err)
}

func TestSaveDose_Valid_Basal_NoDrug(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, d *domain.InsulinDose) error {
			assert.Equal(t, domain.InsulinTypeBasal, d.InsulinType)
			assert.Equal(t, "", d.Drug)
			return nil
		})

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 20.0, domain.InsulinTypeBasal, "")
	require.NoError(t, err)
}

func TestSaveDose_ZeroDose_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 0, domain.InsulinTypeBolus, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dose must be positive")
}

func TestSaveDose_NegativeDose_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, -1, domain.InsulinTypeBolus, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dose must be positive")
}

func TestSaveDose_TooHighDose_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 201, domain.InsulinTypeBolus, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveDose_MaxBoundary_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 200.0, domain.InsulinTypeBolus, "")
	require.NoError(t, err)
}

func TestSaveDose_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	uc := insulin.New(repo)
	err := uc.SaveDose(context.Background(), 1, 10.0, domain.InsulinTypeBolus, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestGetLastDoses(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	expected := []domain.InsulinDose{
		{ID: 1, UserID: 1, DoseUnits: 8, InsulinType: domain.InsulinTypeBolus},
	}
	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(expected, nil)

	uc := insulin.New(repo)
	doses, err := uc.GetLastDoses(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Len(t, doses, 1)
}

func TestGetLastDoses_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInsulinRepository(ctrl)

	repo.EXPECT().GetLast(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := insulin.New(repo)
	_, err := uc.GetLastDoses(context.Background(), 1, 5)
	assert.Error(t, err)
}

func TestIsAnomalousDose(t *testing.T) {
	tests := []struct {
		dose        float64
		insulinType domain.InsulinType
		want        bool
	}{
		{50.0, domain.InsulinTypeBolus, false},
		{50.1, domain.InsulinTypeBolus, true},
		{100.0, domain.InsulinTypeBasal, false},
		{100.1, domain.InsulinTypeBasal, true},
		{30.0, domain.InsulinTypeBolus, false},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := insulin.IsAnomalousDose(tt.dose, tt.insulinType)
			assert.Equal(t, tt.want, result)
		})
	}
}
