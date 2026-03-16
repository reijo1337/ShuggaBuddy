// Package activity содержит бизнес-логику записи физической активности.
package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

const (
	MinDurationMin = 1
	MaxDurationMin = 600
	MaxCustomLen   = 100
)

var validTypes = map[domain.ActivityType]bool{
	domain.ActivityWalking:   true,
	domain.ActivityRunning:   true,
	domain.ActivityCycling:   true,
	domain.ActivityStrength:  true,
	domain.ActivitySwimming:  true,
	domain.ActivityYoga:      true,
	domain.ActivityDancing:   true,
	domain.ActivityTeamSport: true,
	domain.ActivitySkiing:    true,
	domain.ActivityOther:     true,
}

var validIntensities = map[domain.Intensity]bool{
	domain.IntensityLow:    true,
	domain.IntensityMedium: true,
	domain.IntensityHigh:   true,
}

// UseCase содержит бизнес-логику записи активности.
type UseCase struct {
	repo         domain.ActivityRepository
	glucRepo     domain.GlucoseRepository
	reminderRepo domain.ReminderRepository
}

// New создаёт новый UseCase.
func New(repo domain.ActivityRepository, glucRepo domain.GlucoseRepository, reminderRepo domain.ReminderRepository) *UseCase {
	return &UseCase{repo: repo, glucRepo: glucRepo, reminderRepo: reminderRepo}
}

// SaveEntry сохраняет запись активности.
func (uc *UseCase) SaveEntry(ctx context.Context, userID int64, activityType domain.ActivityType, customType string, durationMin int, intensity domain.Intensity, recordedAt time.Time, chatID int64) error {
	if !validTypes[activityType] {
		return fmt.Errorf("activity.SaveEntry: invalid activity type %q", activityType)
	}
	if activityType == domain.ActivityOther {
		if customType == "" {
			return fmt.Errorf("activity.SaveEntry: custom type required for 'other'")
		}
		if len(customType) > MaxCustomLen {
			return fmt.Errorf("activity.SaveEntry: custom type too long (max %d)", MaxCustomLen)
		}
	}
	if durationMin < MinDurationMin || durationMin > MaxDurationMin {
		return fmt.Errorf("activity.SaveEntry: duration %d out of range [%d–%d]", durationMin, MinDurationMin, MaxDurationMin)
	}
	if !validIntensities[intensity] {
		intensity = domain.IntensityMedium
	}

	entry := &domain.ActivityEntry{
		UserID:       userID,
		ActivityType: activityType,
		CustomType:   customType,
		DurationMin:  durationMin,
		Intensity:    intensity,
		RecordedAt:   recordedAt,
	}

	id, err := uc.repo.Save(ctx, entry)
	if err != nil {
		return fmt.Errorf("activity.SaveEntry: %w", err)
	}

	if uc.reminderRepo != nil {
		impact := uc.EvaluateImpact(activityType, durationMin, intensity)
		reminder := &domain.Reminder{
			UserID:     userID,
			ActivityID: id,
			ChatID:     chatID,
			FireAt:     recordedAt.Add(time.Duration(impact.MonitorHours) * time.Hour),
		}
		if err := uc.reminderRepo.Save(ctx, reminder); err != nil {
			return fmt.Errorf("activity.SaveEntry: reminder: %w", err)
		}
	}

	return nil
}

// GetLastEntries возвращает последние записи пользователя.
func (uc *UseCase) GetLastEntries(ctx context.Context, userID int64, limit int) ([]domain.ActivityEntry, error) {
	entries, err := uc.repo.GetLast(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("activity.GetLastEntries: %w", err)
	}
	return entries, nil
}

// EvaluateImpact возвращает оценку влияния активности на гликемию.
func (uc *UseCase) EvaluateImpact(activityType domain.ActivityType, durationMin int, intensity domain.Intensity) domain.GlycemicImpact {
	risk := evaluateRisk(activityType, durationMin)
	risk = applyIntensity(risk, intensity)
	hours := 1
	switch risk {
	case domain.RiskModerate:
		hours = 2
	case domain.RiskHigh:
		hours = 4
	}
	return domain.GlycemicImpact{RiskLevel: risk, MonitorHours: hours}
}

// AnalyzeLastActivities возвращает корреляцию последних активностей с глюкозой.
func (uc *UseCase) AnalyzeLastActivities(ctx context.Context, userID int64, limit int) ([]domain.ActivityAnalysis, error) {
	entries, err := uc.repo.GetLast(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("activity.AnalyzeLastActivities: %w", err)
	}

	analyses := make([]domain.ActivityAnalysis, len(entries))
	for i, e := range entries {
		a := domain.ActivityAnalysis{Entry: e}

		if uc.glucRepo != nil {
			impact := uc.EvaluateImpact(e.ActivityType, e.DurationMin, e.Intensity)

			beforeReadings, err := uc.glucRepo.GetByTimeRange(ctx, userID, e.RecordedAt.Add(-1*time.Hour), e.RecordedAt)
			if err != nil {
				return nil, fmt.Errorf("activity.AnalyzeLastActivities: before glucose: %w", err)
			}
			if len(beforeReadings) > 0 {
				last := beforeReadings[len(beforeReadings)-1]
				a.GlucBefore = &last.ValueMmol
				a.TimeBefore = &last.RecordedAt
			}

			afterReadings, err := uc.glucRepo.GetByTimeRange(ctx, userID, e.RecordedAt, e.RecordedAt.Add(time.Duration(impact.MonitorHours)*time.Hour))
			if err != nil {
				return nil, fmt.Errorf("activity.AnalyzeLastActivities: after glucose: %w", err)
			}
			if len(afterReadings) > 0 {
				last := afterReadings[len(afterReadings)-1]
				a.GlucAfter = &last.ValueMmol
				a.TimeAfter = &last.RecordedAt
			}

			if a.GlucBefore != nil && a.GlucAfter != nil {
				delta := *a.GlucAfter - *a.GlucBefore
				a.Delta = &delta
			}
		}

		analyses[i] = a
	}

	return analyses, nil
}

func applyIntensity(risk domain.RiskLevel, intensity domain.Intensity) domain.RiskLevel {
	levels := []domain.RiskLevel{domain.RiskLow, domain.RiskModerate, domain.RiskHigh}
	idx := 0
	for i, l := range levels {
		if l == risk {
			idx = i
			break
		}
	}

	switch intensity {
	case domain.IntensityHigh:
		if idx < len(levels)-1 {
			idx++
		}
	case domain.IntensityLow:
		if idx > 0 {
			idx--
		}
	}
	return levels[idx]
}

func evaluateRisk(at domain.ActivityType, dur int) domain.RiskLevel {
	type bracket struct {
		short  domain.RiskLevel // < 20
		medium domain.RiskLevel // 20–45
		long   domain.RiskLevel // > 45
	}

	matrix := map[domain.ActivityType]bracket{
		domain.ActivityWalking:   {domain.RiskLow, domain.RiskLow, domain.RiskModerate},
		domain.ActivityRunning:   {domain.RiskModerate, domain.RiskHigh, domain.RiskHigh},
		domain.ActivityCycling:   {domain.RiskLow, domain.RiskModerate, domain.RiskHigh},
		domain.ActivityStrength:  {domain.RiskModerate, domain.RiskModerate, domain.RiskHigh},
		domain.ActivitySwimming:  {domain.RiskModerate, domain.RiskHigh, domain.RiskHigh},
		domain.ActivityYoga:      {domain.RiskLow, domain.RiskLow, domain.RiskLow},
		domain.ActivityDancing:   {domain.RiskLow, domain.RiskModerate, domain.RiskModerate},
		domain.ActivityTeamSport: {domain.RiskModerate, domain.RiskHigh, domain.RiskHigh},
		domain.ActivitySkiing:    {domain.RiskModerate, domain.RiskHigh, domain.RiskHigh},
		domain.ActivityOther:     {domain.RiskLow, domain.RiskModerate, domain.RiskModerate},
	}

	b, ok := matrix[at]
	if !ok {
		return domain.RiskLow
	}

	switch {
	case dur < 20:
		return b.short
	case dur <= 45:
		return b.medium
	default:
		return b.long
	}
}
