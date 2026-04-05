package bolus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
)

// --- IOB Tests ---

func TestCalculateIOB_SingleDose(t *testing.T) {
	now := time.Now()
	dia := 4.0
	doses := []domain.InsulinDose{
		{DoseUnits: 10, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-2 * time.Hour)},
	}
	iob := calculateIOB(doses, dia, now)
	assert.InDelta(t, 5.0, iob, 0.01)
}

func TestCalculateIOB_MultipleDoses(t *testing.T) {
	now := time.Now()
	dia := 4.0
	doses := []domain.InsulinDose{
		{DoseUnits: 10, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-1 * time.Hour)},
		{DoseUnits: 4, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-3 * time.Hour)},
	}
	iob := calculateIOB(doses, dia, now)
	assert.InDelta(t, 8.5, iob, 0.01)
}

func TestCalculateIOB_ExpiredDose(t *testing.T) {
	now := time.Now()
	dia := 4.0
	doses := []domain.InsulinDose{
		{DoseUnits: 10, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-5 * time.Hour)},
	}
	iob := calculateIOB(doses, dia, now)
	assert.InDelta(t, 0.0, iob, 0.01)
}

func TestCalculateIOB_NoDoses(t *testing.T) {
	now := time.Now()
	iob := calculateIOB(nil, 4.0, now)
	assert.InDelta(t, 0.0, iob, 0.01)
}

func TestCalculateIOB_SkipsBasalDoses(t *testing.T) {
	now := time.Now()
	dia := 4.0
	doses := []domain.InsulinDose{
		{DoseUnits: 10, InsulinType: domain.InsulinTypeBasal, RecordedAt: now.Add(-1 * time.Hour)},
		{DoseUnits: 5, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-2 * time.Hour)},
	}
	iob := calculateIOB(doses, dia, now)
	assert.InDelta(t, 2.5, iob, 0.01)
}

// --- ICR Tests ---

func TestDeriveICR_Enough(t *testing.T) {
	now := time.Now()
	targetMin := 3.9
	targetMax := 10.0

	var foods []domain.FoodEntry
	var doses []domain.InsulinDose
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		baseTime := now.Add(-time.Duration(i*6) * time.Hour)
		foods = append(foods, domain.FoodEntry{CarbsGrams: 60, EatenAt: baseTime})
		doses = append(doses, domain.InsulinDose{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: baseTime.Add(5 * time.Minute)})
		glucoseReadings = append(glucoseReadings, domain.GlucoseReading{ValueMmol: 7.0, RecordedAt: baseTime.Add(3 * time.Hour)})
	}

	icr, err := deriveICR(foods, doses, glucoseReadings, targetMin, targetMax)
	assert.NoError(t, err)
	assert.InDelta(t, 10.0, icr, 0.01)
}

func TestDeriveICR_NotEnoughChains(t *testing.T) {
	now := time.Now()
	foods := []domain.FoodEntry{{CarbsGrams: 60, EatenAt: now}}
	doses := []domain.InsulinDose{{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(5 * time.Minute)}}
	glucoseReadings := []domain.GlucoseReading{{ValueMmol: 7.0, RecordedAt: now.Add(3 * time.Hour)}}

	_, err := deriveICR(foods, doses, glucoseReadings, 3.9, 10.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")
}

func TestDeriveICR_OutOfRangeGlucoseExcluded(t *testing.T) {
	now := time.Now()
	var foods []domain.FoodEntry
	var doses []domain.InsulinDose
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 4; i++ {
		baseTime := now.Add(-time.Duration(i*6) * time.Hour)
		foods = append(foods, domain.FoodEntry{CarbsGrams: 60, EatenAt: baseTime})
		doses = append(doses, domain.InsulinDose{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: baseTime.Add(5 * time.Minute)})
		glucoseReadings = append(glucoseReadings, domain.GlucoseReading{ValueMmol: 7.0, RecordedAt: baseTime.Add(3 * time.Hour)})
	}
	for i := 4; i < 7; i++ {
		baseTime := now.Add(-time.Duration(i*6) * time.Hour)
		foods = append(foods, domain.FoodEntry{CarbsGrams: 60, EatenAt: baseTime})
		doses = append(doses, domain.InsulinDose{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: baseTime.Add(5 * time.Minute)})
		glucoseReadings = append(glucoseReadings, domain.GlucoseReading{ValueMmol: 15.0, RecordedAt: baseTime.Add(3 * time.Hour)})
	}

	_, err := deriveICR(foods, doses, glucoseReadings, 3.9, 10.0)
	assert.Error(t, err)
}

// --- ISF Tests ---

func TestDeriveISF_Enough(t *testing.T) {
	now := time.Now()
	var foods []domain.FoodEntry
	var doses []domain.InsulinDose
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		bolusTime := now.Add(-time.Duration(i*6) * time.Hour)
		doses = append(doses, domain.InsulinDose{DoseUnits: 2, InsulinType: domain.InsulinTypeBolus, RecordedAt: bolusTime})
		glucoseReadings = append(glucoseReadings,
			domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: bolusTime.Add(-10 * time.Minute)},
			domain.GlucoseReading{ValueMmol: 8.0, RecordedAt: bolusTime.Add(3 * time.Hour)},
		)
	}

	isf, err := deriveISF(foods, doses, glucoseReadings)
	assert.NoError(t, err)
	assert.InDelta(t, 2.0, isf, 0.01)
}

func TestDeriveISF_NotEnoughChains(t *testing.T) {
	now := time.Now()
	doses := []domain.InsulinDose{{DoseUnits: 2, InsulinType: domain.InsulinTypeBolus, RecordedAt: now}}
	glucoseReadings := []domain.GlucoseReading{
		{ValueMmol: 12.0, RecordedAt: now.Add(-10 * time.Minute)},
		{ValueMmol: 8.0, RecordedAt: now.Add(3 * time.Hour)},
	}
	_, err := deriveISF(nil, doses, glucoseReadings)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")
}

func TestDeriveISF_SkipsDosesWithFood(t *testing.T) {
	now := time.Now()
	var foods []domain.FoodEntry
	var doses []domain.InsulinDose
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		bolusTime := now.Add(-time.Duration(i*6) * time.Hour)
		doses = append(doses, domain.InsulinDose{DoseUnits: 2, InsulinType: domain.InsulinTypeBolus, RecordedAt: bolusTime})
		foods = append(foods, domain.FoodEntry{CarbsGrams: 40, EatenAt: bolusTime.Add(-5 * time.Minute)})
		glucoseReadings = append(glucoseReadings,
			domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: bolusTime.Add(-10 * time.Minute)},
			domain.GlucoseReading{ValueMmol: 8.0, RecordedAt: bolusTime.Add(3 * time.Hour)},
		)
	}
	_, err := deriveISF(foods, doses, glucoseReadings)
	assert.Error(t, err)
}

// --- Calculate Tests ---

func TestCalculate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)

	uc := New(userRepo, insulinRepo, glucoseRepo, foodRepo)
	ctx := context.Background()
	now := time.Now()

	user := &domain.User{
		ID: 1, BolusDrug: "novorapid",
		TargetMinMmol: 4.0, TargetMaxMmol: 10.0,
	}

	userRepo.EXPECT().GetByID(ctx, int64(1)).Return(user, nil)

	var doses []*domain.InsulinDose
	var foods []*domain.FoodEntry
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		base := now.Add(-time.Duration(i*6) * time.Hour)
		foods = append(foods, &domain.FoodEntry{CarbsGrams: 60, EatenAt: base})
		doses = append(doses, &domain.InsulinDose{
			DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: base.Add(5 * time.Minute),
		})
		glucoseReadings = append(glucoseReadings, domain.GlucoseReading{
			ValueMmol: 7.0, RecordedAt: base.Add(3 * time.Hour),
		})
	}

	for i := 5; i < 10; i++ {
		base := now.Add(-time.Duration(i*6) * time.Hour)
		doses = append(doses, &domain.InsulinDose{
			DoseUnits: 2, InsulinType: domain.InsulinTypeBolus, RecordedAt: base,
		})
		glucoseReadings = append(glucoseReadings,
			domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: base.Add(-10 * time.Minute)},
			domain.GlucoseReading{ValueMmol: 8.0, RecordedAt: base.Add(3 * time.Hour)},
		)
	}

	doses = append(doses, &domain.InsulinDose{
		DoseUnits: 5, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-1 * time.Hour),
	})

	insulinRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(doses, nil)
	foodRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(foods, nil)
	glucoseRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(glucoseReadings, nil)

	// ICR=10, ISF=2.0, targetMid=7.0
	// foodDose=60/10=6, correction=(9-7)/2=1, IOB=5*(1-1/4)=3.75
	// total=max(0, 6+1-3.75)=3.25
	rec, err := uc.Calculate(ctx, 1, 9.0, 60.0, now)
	assert.NoError(t, err)
	assert.InDelta(t, 6.0, rec.FoodDose, 0.1)
	assert.InDelta(t, 1.0, rec.CorrectionDose, 0.1)
	assert.InDelta(t, 3.75, rec.IOB, 0.1)
	assert.InDelta(t, 3.25, rec.TotalDose, 0.1)
}

func TestCalculate_NoBolusDrug(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)

	uc := New(userRepo, insulinRepo, glucoseRepo, foodRepo)
	userRepo.EXPECT().GetByID(gomock.Any(), int64(1)).Return(&domain.User{ID: 1, BolusDrug: ""}, nil)

	_, err := uc.Calculate(context.Background(), 1, 9.0, 60.0, time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bolus drug not set")
}

func TestCalculate_TotalFlooredToZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)

	uc := New(userRepo, insulinRepo, glucoseRepo, foodRepo)
	ctx := context.Background()
	now := time.Now()

	user := &domain.User{
		ID: 1, BolusDrug: "novorapid",
		TargetMinMmol: 4.0, TargetMaxMmol: 10.0,
	}
	userRepo.EXPECT().GetByID(ctx, int64(1)).Return(user, nil)

	var doses []*domain.InsulinDose
	var foods []*domain.FoodEntry
	var glucoseReadings []domain.GlucoseReading

	for i := 0; i < 5; i++ {
		base := now.Add(-time.Duration(i*6) * time.Hour)
		foods = append(foods, &domain.FoodEntry{CarbsGrams: 60, EatenAt: base})
		doses = append(doses, &domain.InsulinDose{DoseUnits: 6, InsulinType: domain.InsulinTypeBolus, RecordedAt: base.Add(5 * time.Minute)})
		glucoseReadings = append(glucoseReadings, domain.GlucoseReading{ValueMmol: 7.0, RecordedAt: base.Add(3 * time.Hour)})
	}
	for i := 5; i < 10; i++ {
		base := now.Add(-time.Duration(i*6) * time.Hour)
		doses = append(doses, &domain.InsulinDose{DoseUnits: 2, InsulinType: domain.InsulinTypeBolus, RecordedAt: base})
		glucoseReadings = append(glucoseReadings,
			domain.GlucoseReading{ValueMmol: 12.0, RecordedAt: base.Add(-10 * time.Minute)},
			domain.GlucoseReading{ValueMmol: 8.0, RecordedAt: base.Add(3 * time.Hour)},
		)
	}
	doses = append(doses, &domain.InsulinDose{DoseUnits: 50, InsulinType: domain.InsulinTypeBolus, RecordedAt: now.Add(-30 * time.Minute)})

	insulinRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(doses, nil)
	foodRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(foods, nil)
	glucoseRepo.EXPECT().GetByTimeRange(ctx, int64(1), gomock.Any(), gomock.Any()).Return(glucoseReadings, nil)

	rec, err := uc.Calculate(ctx, 1, 5.0, 10.0, now)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, rec.TotalDose)
}
