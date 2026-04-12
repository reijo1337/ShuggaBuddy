package doseadvisor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
)

func newTestUC(ctrl *gomock.Controller) (*UseCase, *mocks.MockUserRepository, *mocks.MockInsulinRepository, *mocks.MockGlucoseRepository, *mocks.MockFoodRepository) {
	userRepo := mocks.NewMockUserRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)
	uc := New(userRepo, insulinRepo, glucoseRepo, foodRepo)
	return uc, userRepo, insulinRepo, glucoseRepo, foodRepo
}

func TestAnalyzeBasal_TrendHigh(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	readings := make([]domain.GlucoseReading, 0, 7)
	for i := range 7 {
		day := now.AddDate(0, 0, -i)
		morning := time.Date(day.Year(), day.Month(), day.Day(), 7, 0, 0, 0, time.UTC)
		readings = append(readings, domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: morning})
	}
	// Analyze calls glucoseRepo twice (basal + bolus) and foodRepo twice
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil).Times(2)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice.Basal)
	assert.Equal(t, domain.TrendHigh, advice.Basal.Trend)
	assert.InDelta(t, 12.0, advice.Basal.FastingAvg, 0.1)
	assert.Equal(t, 7, advice.Basal.FastingCount)
	assert.Equal(t, 18.0, advice.Basal.CurrentDose)
	assert.Greater(t, advice.Basal.SuggestedDose, advice.Basal.CurrentDose)
}

func TestAnalyzeBasal_TrendLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	var readings []domain.GlucoseReading
	for i := 0; i < 5; i++ {
		day := now.AddDate(0, 0, -i)
		morning := time.Date(day.Year(), day.Month(), day.Day(), 7, 0, 0, 0, time.UTC)
		readings = append(readings, domain.GlucoseReading{ValueMmol: 3.0, RecordedAt: morning})
	}
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil).Times(2)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice.Basal)
	assert.Equal(t, domain.TrendLow, advice.Basal.Trend)
	assert.Less(t, advice.Basal.SuggestedDose, advice.Basal.CurrentDose)
}

func TestAnalyzeBasal_TrendStable(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	var readings []domain.GlucoseReading
	for i := 0; i < 5; i++ {
		day := now.AddDate(0, 0, -i)
		morning := time.Date(day.Year(), day.Month(), day.Day(), 7, 0, 0, 0, time.UTC)
		readings = append(readings, domain.GlucoseReading{ValueMmol: 6.5, RecordedAt: morning})
	}
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil).Times(2)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice.Basal)
	assert.Equal(t, domain.TrendStable, advice.Basal.Trend)
	assert.Equal(t, 18.0, advice.Basal.SuggestedDose)
}

func TestAnalyzeBasal_NoDose(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{ID: 1, BasalDose: 0, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, Timezone: "UTC"}
	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	assert.Nil(t, advice.Basal)
	assert.Nil(t, advice.Bolus)
}

func TestAnalyzeBasal_InsufficientData(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	var readings []domain.GlucoseReading
	for i := 0; i < 2; i++ {
		day := now.AddDate(0, 0, -i)
		morning := time.Date(day.Year(), day.Month(), day.Day(), 7, 0, 0, 0, time.UTC)
		readings = append(readings, domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: morning})
	}
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil).Times(2)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	assert.Nil(t, advice.Basal)
}

func TestAnalyzeBolus_ICRIncreased(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 0, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, BolusDrug: "novorapid", Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	var foods []*domain.FoodEntry
	var doses []*domain.InsulinDose
	var readings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		base := now.AddDate(0, 0, -13+i)
		foods = append(foods, &domain.FoodEntry{CarbsGrams: 60, EatenAt: base})
		doses = append(doses, &domain.InsulinDose{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: base.Add(5 * time.Minute)})
		readings = append(readings, domain.GlucoseReading{ValueMmol: 7.0, RecordedAt: base.Add(3 * time.Hour)})
	}
	for i := 0; i < 5; i++ {
		base := now.AddDate(0, 0, -6+i)
		foods = append(foods, &domain.FoodEntry{CarbsGrams: 60, EatenAt: base})
		doses = append(doses, &domain.InsulinDose{DoseUnits: 7.5, InsulinType: domain.InsulinTypeBolus, RecordedAt: base.Add(5 * time.Minute)})
		readings = append(readings, domain.GlucoseReading{ValueMmol: 7.0, RecordedAt: base.Add(3 * time.Hour)})
	}

	// BasalDose=0, so basal skips repo calls; only bolus calls glucose+food+insulin
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(foods, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(doses, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice.Bolus)
	assert.Equal(t, domain.TrendHigh, advice.Bolus.ICRTrend)
}

func TestAnalyzeBolus_InsufficientData(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 0, TargetMinMmol: 3.9, TargetMaxMmol: 10.0, BolusDrug: "novorapid", Timezone: "UTC",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)
	// BasalDose=0, so basal skips repo calls; only bolus calls glucose+food+insulin
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	assert.Nil(t, advice.Bolus)
}

func TestAnalyze_BothAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 18, TargetMinMmol: 3.9, TargetMaxMmol: 10.0,
		Timezone: "UTC", BolusDrug: "novorapid",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)

	var readings []domain.GlucoseReading
	for i := 0; i < 5; i++ {
		day := now.AddDate(0, 0, -i)
		morning := time.Date(day.Year(), day.Month(), day.Day(), 7, 0, 0, 0, time.UTC)
		readings = append(readings, domain.GlucoseReading{ValueMmol: 6.5, RecordedAt: morning})
	}
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(readings, nil).Times(2)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice)
	assert.NotNil(t, advice.Basal)
	assert.Nil(t, advice.Bolus)
}

func TestAnalyze_NeitherAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, userRepo, insulinRepo, glucoseRepo, foodRepo := newTestUC(ctrl)

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		ID: 1, BasalDose: 0, TargetMinMmol: 3.9, TargetMaxMmol: 10.0,
		Timezone: "UTC", BolusDrug: "novorapid",
	}

	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(user, nil)
	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil)

	advice, err := uc.Analyze(context.Background(), 1, now)
	require.NoError(t, err)
	require.NotNil(t, advice)
	assert.Nil(t, advice.Basal)
	assert.Nil(t, advice.Bolus)
}
