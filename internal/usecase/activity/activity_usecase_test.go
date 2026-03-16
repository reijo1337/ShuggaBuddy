package activity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/activity"
)

func TestSaveEntry_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, e *domain.ActivityEntry) (int64, error) {
			assert.Equal(t, int64(1), e.UserID)
			assert.Equal(t, domain.ActivityRunning, e.ActivityType)
			assert.Equal(t, "", e.CustomType)
			assert.Equal(t, 30, e.DurationMin)
			assert.Equal(t, domain.IntensityMedium, e.Intensity)
			return 42, nil
		})

	reminderRepo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *domain.Reminder) error {
			assert.Equal(t, int64(1), r.UserID)
			assert.Equal(t, int64(42), r.ActivityID)
			assert.Equal(t, int64(456), r.ChatID)
			return nil
		})

	uc := activity.New(repo, nil, reminderRepo)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 30, domain.IntensityMedium, time.Now(), 456)
	require.NoError(t, err)
}

func TestSaveEntry_Other_WithCustomType(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), nil)
	reminderRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := activity.New(repo, nil, reminderRepo)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityOther, "Фехтование", 45, domain.IntensityMedium, time.Now(), 456)
	require.NoError(t, err)
}

func TestSaveEntry_DurationZero_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 0, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveEntry_DurationOne_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), nil)
	reminderRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := activity.New(repo, nil, reminderRepo)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 1, domain.IntensityMedium, time.Now(), 456)
	require.NoError(t, err)
}

func TestSaveEntry_Duration600_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), nil)
	reminderRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := activity.New(repo, nil, reminderRepo)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 600, domain.IntensityMedium, time.Now(), 456)
	require.NoError(t, err)
}

func TestSaveEntry_Duration601_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 601, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveEntry_InvalidType_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityType("invalid"), "", 30, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid activity type")
}

func TestSaveEntry_AllValidTypes(t *testing.T) {
	types := []domain.ActivityType{
		domain.ActivityWalking, domain.ActivityRunning, domain.ActivityCycling,
		domain.ActivityStrength, domain.ActivitySwimming, domain.ActivityYoga,
		domain.ActivityDancing, domain.ActivityTeamSport, domain.ActivitySkiing,
		domain.ActivityOther,
	}
	for _, at := range types {
		t.Run(string(at), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockActivityRepository(ctrl)
			reminderRepo := mocks.NewMockReminderRepository(ctrl)
			repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			reminderRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

			uc := activity.New(repo, nil, reminderRepo)
			custom := ""
			if at == domain.ActivityOther {
				custom = "Test"
			}
			err := uc.SaveEntry(context.Background(), 1, at, custom, 30, domain.IntensityMedium, time.Now(), 456)
			require.NoError(t, err)
		})
	}
}

func TestSaveEntry_Other_EmptyCustom_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityOther, "", 30, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "custom type required")
}

func TestSaveEntry_Other_TooLongCustom_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	uc := activity.New(repo, nil, nil)
	longStr := string(make([]byte, 101))
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityOther, longStr, 30, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "custom type too long")
}

func TestSaveEntry_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("db error"))

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 30, domain.IntensityMedium, time.Now(), 456)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestSaveEntry_NilReminderRepo_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(int64(1), nil)

	uc := activity.New(repo, nil, nil)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 30, domain.IntensityMedium, time.Now(), 456)
	require.NoError(t, err)
}

func TestSaveEntry_InvalidIntensity_DefaultsMedium(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	reminderRepo := mocks.NewMockReminderRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, e *domain.ActivityEntry) (int64, error) {
			assert.Equal(t, domain.IntensityMedium, e.Intensity)
			return 1, nil
		})
	reminderRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := activity.New(repo, nil, reminderRepo)
	err := uc.SaveEntry(context.Background(), 1, domain.ActivityRunning, "", 30, domain.Intensity("invalid"), time.Now(), 456)
	require.NoError(t, err)
}

func TestGetLastEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)

	expected := []domain.ActivityEntry{
		{ID: 1, UserID: 1, ActivityType: domain.ActivityRunning, DurationMin: 30},
	}
	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(expected, nil)

	uc := activity.New(repo, nil, nil)
	entries, err := uc.GetLastEntries(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestGetLastEntries_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	repo.EXPECT().GetLast(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := activity.New(repo, nil, nil)
	_, err := uc.GetLastEntries(context.Background(), 1, 5)
	assert.Error(t, err)
}

func TestEvaluateImpact(t *testing.T) {
	tests := []struct {
		name     string
		aType    domain.ActivityType
		duration int
		wantRisk domain.RiskLevel
		wantHrs  int
	}{
		// Walking
		{"walking_10min", domain.ActivityWalking, 10, domain.RiskLow, 1},
		{"walking_30min", domain.ActivityWalking, 30, domain.RiskLow, 1},
		{"walking_60min", domain.ActivityWalking, 60, domain.RiskModerate, 2},
		// Running
		{"running_10min", domain.ActivityRunning, 10, domain.RiskModerate, 2},
		{"running_30min", domain.ActivityRunning, 30, domain.RiskHigh, 4},
		{"running_60min", domain.ActivityRunning, 60, domain.RiskHigh, 4},
		// Cycling
		{"cycling_10min", domain.ActivityCycling, 10, domain.RiskLow, 1},
		{"cycling_30min", domain.ActivityCycling, 30, domain.RiskModerate, 2},
		{"cycling_60min", domain.ActivityCycling, 60, domain.RiskHigh, 4},
		// Strength
		{"strength_10min", domain.ActivityStrength, 10, domain.RiskModerate, 2},
		{"strength_30min", domain.ActivityStrength, 30, domain.RiskModerate, 2},
		{"strength_60min", domain.ActivityStrength, 60, domain.RiskHigh, 4},
		// Swimming
		{"swimming_10min", domain.ActivitySwimming, 10, domain.RiskModerate, 2},
		{"swimming_30min", domain.ActivitySwimming, 30, domain.RiskHigh, 4},
		{"swimming_60min", domain.ActivitySwimming, 60, domain.RiskHigh, 4},
		// Yoga
		{"yoga_10min", domain.ActivityYoga, 10, domain.RiskLow, 1},
		{"yoga_30min", domain.ActivityYoga, 30, domain.RiskLow, 1},
		{"yoga_60min", domain.ActivityYoga, 60, domain.RiskLow, 1},
		// Dancing
		{"dancing_10min", domain.ActivityDancing, 10, domain.RiskLow, 1},
		{"dancing_30min", domain.ActivityDancing, 30, domain.RiskModerate, 2},
		{"dancing_60min", domain.ActivityDancing, 60, domain.RiskModerate, 2},
		// Team sport
		{"team_sport_10min", domain.ActivityTeamSport, 10, domain.RiskModerate, 2},
		{"team_sport_30min", domain.ActivityTeamSport, 30, domain.RiskHigh, 4},
		{"team_sport_60min", domain.ActivityTeamSport, 60, domain.RiskHigh, 4},
		// Skiing
		{"skiing_10min", domain.ActivitySkiing, 10, domain.RiskModerate, 2},
		{"skiing_30min", domain.ActivitySkiing, 30, domain.RiskHigh, 4},
		{"skiing_60min", domain.ActivitySkiing, 60, domain.RiskHigh, 4},
		// Other
		{"other_10min", domain.ActivityOther, 10, domain.RiskLow, 1},
		{"other_30min", domain.ActivityOther, 30, domain.RiskModerate, 2},
		{"other_60min", domain.ActivityOther, 60, domain.RiskModerate, 2},
		// Boundary: exactly 20 and 45
		{"running_19min", domain.ActivityRunning, 19, domain.RiskModerate, 2},
		{"running_20min", domain.ActivityRunning, 20, domain.RiskHigh, 4},
		{"walking_45min", domain.ActivityWalking, 45, domain.RiskLow, 1},
		{"walking_46min", domain.ActivityWalking, 46, domain.RiskModerate, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := activity.New(nil, nil, nil)
			impact := uc.EvaluateImpact(tt.aType, tt.duration, domain.IntensityMedium)
			assert.Equal(t, tt.wantRisk, impact.RiskLevel, "risk level")
			assert.Equal(t, tt.wantHrs, impact.MonitorHours, "monitor hours")
		})
	}
}

func TestEvaluateImpact_WithIntensity(t *testing.T) {
	tests := []struct {
		name      string
		aType     domain.ActivityType
		duration  int
		intensity domain.Intensity
		wantRisk  domain.RiskLevel
		wantHrs   int
	}{
		{"walking_30_high", domain.ActivityWalking, 30, domain.IntensityHigh, domain.RiskModerate, 2},
		{"walking_30_low", domain.ActivityWalking, 30, domain.IntensityLow, domain.RiskLow, 1},
		{"walking_30_medium", domain.ActivityWalking, 30, domain.IntensityMedium, domain.RiskLow, 1},
		{"running_10_high", domain.ActivityRunning, 10, domain.IntensityHigh, domain.RiskHigh, 4},
		{"running_10_low", domain.ActivityRunning, 10, domain.IntensityLow, domain.RiskLow, 1},
		{"running_30_high", domain.ActivityRunning, 30, domain.IntensityHigh, domain.RiskHigh, 4},
		{"running_30_low", domain.ActivityRunning, 30, domain.IntensityLow, domain.RiskModerate, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := activity.New(nil, nil, nil)
			impact := uc.EvaluateImpact(tt.aType, tt.duration, tt.intensity)
			assert.Equal(t, tt.wantRisk, impact.RiskLevel, "risk level")
			assert.Equal(t, tt.wantHrs, impact.MonitorHours, "monitor hours")
		})
	}
}

func TestAnalyzeLastActivities(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	glucRepo := mocks.NewMockGlucoseRepository(ctrl)

	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	entries := []domain.ActivityEntry{
		{
			ID: 1, UserID: 1, ActivityType: domain.ActivityRunning,
			DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: now,
		},
	}

	glucBefore := []domain.GlucoseReading{
		{ValueMmol: 6.0, RecordedAt: now.Add(-20 * time.Minute)},
	}
	glucAfter := []domain.GlucoseReading{
		{ValueMmol: 4.5, RecordedAt: now.Add(90 * time.Minute)},
	}

	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(entries, nil)

	glucRepo.EXPECT().
		GetByTimeRange(gomock.Any(), int64(1), now.Add(-1*time.Hour), now).
		Return(glucBefore, nil)

	glucRepo.EXPECT().
		GetByTimeRange(gomock.Any(), int64(1), now, now.Add(4*time.Hour)).
		Return(glucAfter, nil)

	uc := activity.New(repo, glucRepo, nil)
	analyses, err := uc.AnalyzeLastActivities(context.Background(), 1, 5)
	require.NoError(t, err)
	require.Len(t, analyses, 1)

	a := analyses[0]
	assert.Equal(t, int64(1), a.Entry.ID)
	require.NotNil(t, a.GlucBefore)
	assert.InDelta(t, 6.0, *a.GlucBefore, 0.01)
	require.NotNil(t, a.GlucAfter)
	assert.InDelta(t, 4.5, *a.GlucAfter, 0.01)
	require.NotNil(t, a.Delta)
	assert.InDelta(t, -1.5, *a.Delta, 0.01)
}

func TestAnalyzeLastActivities_NoGlucose(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	glucRepo := mocks.NewMockGlucoseRepository(ctrl)

	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	entries := []domain.ActivityEntry{
		{
			ID: 1, UserID: 1, ActivityType: domain.ActivityYoga,
			DurationMin: 30, Intensity: domain.IntensityMedium, RecordedAt: now,
		},
	}

	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(entries, nil)
	glucRepo.EXPECT().GetByTimeRange(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)

	uc := activity.New(repo, glucRepo, nil)
	analyses, err := uc.AnalyzeLastActivities(context.Background(), 1, 5)
	require.NoError(t, err)
	require.Len(t, analyses, 1)

	a := analyses[0]
	assert.Nil(t, a.GlucBefore)
	assert.Nil(t, a.GlucAfter)
	assert.Nil(t, a.Delta)
}
