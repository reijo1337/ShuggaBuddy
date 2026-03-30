// Package diary contains business logic for the mixed diary feed.
package diary

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// DiaryUseCase is the interface for diary business logic.
type DiaryUseCase interface {
	GetDayEntries(ctx context.Context, userID int64, date time.Time, loc *time.Location) ([]*domain.DiaryEntry, error)
}

// UseCase implements DiaryUseCase.
type UseCase struct {
	glucose  domain.GlucoseRepository
	food     domain.FoodRepository
	insulin  domain.InsulinRepository
	activity domain.ActivityRepository
	note     domain.NoteRepository
}

// New creates a new diary UseCase.
func New(
	glucose domain.GlucoseRepository,
	food domain.FoodRepository,
	insulin domain.InsulinRepository,
	activity domain.ActivityRepository,
	note domain.NoteRepository,
) *UseCase {
	return &UseCase{
		glucose:  glucose,
		food:     food,
		insulin:  insulin,
		activity: activity,
		note:     note,
	}
}

// GetDayEntries returns all diary entries for the given user on the given calendar day in loc.
func (uc *UseCase) GetDayEntries(ctx context.Context, userID int64, date time.Time, loc *time.Location) ([]*domain.DiaryEntry, error) {
	if loc == nil {
		loc = time.UTC
	}
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	to := from.Add(24 * time.Hour)

	entries := make([]*domain.DiaryEntry, 0)

	glucoseReadings, err := uc.glucose.GetByTimeRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("diary.GetDayEntries glucose: %w", err)
	}
	for i := range glucoseReadings {
		r := glucoseReadings[i]
		entries = append(entries, &domain.DiaryEntry{
			Kind:    domain.DiaryKindGlucose,
			Time:    r.RecordedAt,
			Glucose: &r,
		})
	}

	foodEntries, err := uc.food.GetByTimeRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("diary.GetDayEntries food: %w", err)
	}
	for _, f := range foodEntries {
		entries = append(entries, &domain.DiaryEntry{
			Kind: domain.DiaryKindFood,
			Time: f.EatenAt,
			Food: f,
		})
	}

	insulinDoses, err := uc.insulin.GetByTimeRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("diary.GetDayEntries insulin: %w", err)
	}
	for _, d := range insulinDoses {
		entries = append(entries, &domain.DiaryEntry{
			Kind:    domain.DiaryKindInsulin,
			Time:    d.RecordedAt,
			Insulin: d,
		})
	}

	activityEntries, err := uc.activity.GetByTimeRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("diary.GetDayEntries activity: %w", err)
	}
	for _, a := range activityEntries {
		entries = append(entries, &domain.DiaryEntry{
			Kind:     domain.DiaryKindActivity,
			Time:     a.RecordedAt,
			Activity: a,
		})
	}

	noteEntries, err := uc.note.GetByTimeRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("diary.GetDayEntries note: %w", err)
	}
	for _, n := range noteEntries {
		entries = append(entries, &domain.DiaryEntry{
			Kind: domain.DiaryKindNote,
			Time: n.CreatedAt,
			Note: n,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.Before(entries[j].Time)
	})

	return entries, nil
}
