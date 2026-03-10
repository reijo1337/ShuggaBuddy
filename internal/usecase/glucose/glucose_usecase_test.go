package glucose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/glucose"
)

func TestSaveReading_Mmol_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *domain.GlucoseReading) error {
			assert.InDelta(t, 5.4, r.ValueMmol, 0.01)
			assert.Equal(t, "manual", r.Source)
			assert.Equal(t, int64(1), r.UserID)
			return nil
		})

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 5.4, domain.UnitsMmol)
	require.NoError(t, err)
}

func TestSaveReading_Mmol_TooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 0.5, domain.UnitsMmol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveReading_Mmol_TooHigh(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 35.0, domain.UnitsMmol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveReading_Mgdl_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *domain.GlucoseReading) error {
			// 100 mg/dL ≈ 5.55 mmol/L
			assert.InDelta(t, 100.0/domain.MmolToMgdl, r.ValueMmol, 0.01)
			return nil
		})

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 100.0, domain.UnitsMgdl)
	require.NoError(t, err)
}

func TestSaveReading_Mgdl_TooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 10.0, domain.UnitsMgdl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveReading_Mgdl_TooHigh(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 700.0, domain.UnitsMgdl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveReading_InvalidUnits(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 5.0, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown units")
}

func TestSaveReading_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	uc := glucose.New(repo)
	err := uc.SaveReading(context.Background(), 1, 5.4, domain.UnitsMmol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestSaveReading_BoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		units domain.Units
	}{
		{"min mmol", 1.0, domain.UnitsMmol},
		{"max mmol", 33.3, domain.UnitsMmol},
		{"min mgdl", 18.0, domain.UnitsMgdl},
		{"max mgdl", 600.0, domain.UnitsMgdl},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockGlucoseRepository(ctrl)

			repo.EXPECT().
				Save(gomock.Any(), gomock.Any()).
				Return(nil)

			uc := glucose.New(repo)
			err := uc.SaveReading(context.Background(), 1, tt.value, tt.units)
			assert.NoError(t, err)
		})
	}
}

func TestGetLastReadings(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	expected := []domain.GlucoseReading{
		{ID: 1, UserID: 1, ValueMmol: 5.4},
		{ID: 2, UserID: 1, ValueMmol: 6.1},
	}

	repo.EXPECT().
		GetLast(gomock.Any(), int64(1), 5).
		Return(expected, nil)

	uc := glucose.New(repo)
	readings, err := uc.GetLastReadings(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Len(t, readings, 2)
	assert.Equal(t, 5.4, readings[0].ValueMmol)
}

func TestGetLastReadings_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	repo.EXPECT().
		GetLast(gomock.Any(), int64(1), 5).
		Return(nil, nil)

	uc := glucose.New(repo)
	readings, err := uc.GetLastReadings(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestGetLastReadings_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockGlucoseRepository(ctrl)

	repo.EXPECT().
		GetLast(gomock.Any(), int64(1), 5).
		Return(nil, errors.New("db error"))

	uc := glucose.New(repo)
	_, err := uc.GetLastReadings(context.Background(), 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestFormatValue_Mmol(t *testing.T) {
	result := glucose.FormatValue(5.4, domain.UnitsMmol)
	assert.Equal(t, "5.4", result)
}

func TestFormatValue_Mgdl(t *testing.T) {
	result := glucose.FormatValue(5.55, domain.UnitsMgdl)
	// 5.55 * 18.0182 ≈ 100
	assert.Equal(t, "100", result)
}
