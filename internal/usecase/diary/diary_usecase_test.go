package diary_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/diary"
)

func TestGetDayEntries_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	noteRepo := mocks.NewMockNoteRepository(ctrl)

	date := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	userID := int64(42)

	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	activityRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	noteRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)

	uc := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)
	entries, err := uc.GetDayEntries(context.Background(), userID, date, time.UTC)

	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries)
}

func TestGetDayEntries_GlucoseAndFood_SortedByTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	noteRepo := mocks.NewMockNoteRepository(ctrl)

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	userID := int64(42)

	glucoseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	foodTime := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC) // earlier

	glucoseReadings := []domain.GlucoseReading{
		{ID: 1, UserID: userID, ValueMmol: 6.5, RecordedAt: glucoseTime},
	}
	foodEntries := []*domain.FoodEntry{
		{ID: 1, UserID: userID, CarbsGrams: 30, EatenAt: foodTime},
	}

	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(glucoseReadings, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(foodEntries, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	activityRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)
	noteRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, nil)

	uc := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)
	entries, err := uc.GetDayEntries(context.Background(), userID, date, time.UTC)

	require.NoError(t, err)
	require.Len(t, entries, 2)

	// sorted ascending: food (8:00) then glucose (10:00)
	assert.Equal(t, domain.DiaryKindFood, entries[0].Kind)
	assert.Equal(t, foodTime, entries[0].Time)
	assert.NotNil(t, entries[0].Food)

	assert.Equal(t, domain.DiaryKindGlucose, entries[1].Kind)
	assert.Equal(t, glucoseTime, entries[1].Time)
	assert.NotNil(t, entries[1].Glucose)
}

func TestGetDayEntries_AllFiveTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	noteRepo := mocks.NewMockNoteRepository(ctrl)

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	userID := int64(42)

	t1 := time.Date(2024, 1, 15, 7, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	t4 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	glucoseReadings := []domain.GlucoseReading{
		{ID: 1, UserID: userID, ValueMmol: 6.5, RecordedAt: t3},
	}
	foodEntries := []*domain.FoodEntry{
		{ID: 2, UserID: userID, CarbsGrams: 30, EatenAt: t1},
	}
	insulinDoses := []*domain.InsulinDose{
		{ID: 3, UserID: userID, DoseUnits: 4, InsulinType: domain.InsulinTypeBolus, RecordedAt: t4},
	}
	activityEntries := []*domain.ActivityEntry{
		{ID: 4, UserID: userID, ActivityType: domain.ActivityWalking, DurationMin: 30, RecordedAt: t2},
	}
	noteEntries := []*domain.NoteEntry{
		{ID: 5, UserID: userID, Type: domain.NoteTypeFree, CreatedAt: t5},
	}

	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(glucoseReadings, nil)
	foodRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(foodEntries, nil)
	insulinRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(insulinDoses, nil)
	activityRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(activityEntries, nil)
	noteRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(noteEntries, nil)

	uc := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)
	entries, err := uc.GetDayEntries(context.Background(), userID, date, time.UTC)

	require.NoError(t, err)
	require.Len(t, entries, 5)

	// Sorted ascending by time: t1(food), t2(activity), t3(glucose), t4(insulin), t5(note)
	assert.Equal(t, domain.DiaryKindFood, entries[0].Kind)
	assert.Equal(t, domain.DiaryKindActivity, entries[1].Kind)
	assert.Equal(t, domain.DiaryKindGlucose, entries[2].Kind)
	assert.Equal(t, domain.DiaryKindInsulin, entries[3].Kind)
	assert.Equal(t, domain.DiaryKindNote, entries[4].Kind)
}

func TestGetDayEntries_RepoError_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	glucoseRepo := mocks.NewMockGlucoseRepository(ctrl)
	foodRepo := mocks.NewMockFoodRepository(ctrl)
	insulinRepo := mocks.NewMockInsulinRepository(ctrl)
	activityRepo := mocks.NewMockActivityRepository(ctrl)
	noteRepo := mocks.NewMockNoteRepository(ctrl)

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	userID := int64(42)

	repoErr := errors.New("database error")

	glucoseRepo.EXPECT().GetByTimeRange(gomock.Any(), userID, from, to).Return(nil, repoErr)

	uc := diary.New(glucoseRepo, foodRepo, insulinRepo, activityRepo, noteRepo)
	entries, err := uc.GetDayEntries(context.Background(), userID, date, time.UTC)

	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.ErrorIs(t, err, repoErr)
}
