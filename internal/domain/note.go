package domain

import (
	"context"
	"time"
)

// NoteType defines the category of a diary note.
type NoteType string

const (
	NoteTypeWellbeing NoteType = "wellbeing"
	NoteTypeIllness   NoteType = "illness"
	NoteTypeStress    NoteType = "stress"
	NoteTypeFree      NoteType = "free"
)

// WellbeingValue describes the user's self-reported wellbeing.
type WellbeingValue string

const (
	WellbeingGood   WellbeingValue = "good"
	WellbeingNormal WellbeingValue = "normal"
	WellbeingBad    WellbeingValue = "bad"
	WellbeingSick   WellbeingValue = "sick"
)

// NoteEntry represents a single diary/note record.
type NoteEntry struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	Type      NoteType        `json:"type"`
	Wellbeing *WellbeingValue `json:"wellbeing,omitempty"`
	Text      *string         `json:"text,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// DiaryEntryKind identifies the type of a diary feed entry.
type DiaryEntryKind string

const (
	DiaryKindGlucose  DiaryEntryKind = "glucose"
	DiaryKindFood     DiaryEntryKind = "food"
	DiaryKindInsulin  DiaryEntryKind = "insulin"
	DiaryKindActivity DiaryEntryKind = "activity"
	DiaryKindNote     DiaryEntryKind = "note"
)

// DiaryEntry is a unified record for the mixed diary feed.
type DiaryEntry struct {
	Kind     DiaryEntryKind
	Time     time.Time
	Glucose  *GlucoseReading
	Food     *FoodEntry
	Insulin  *InsulinDose
	Activity *ActivityEntry
	Note     *NoteEntry
}

// GlucoseStatus returns a status string based on the value relative to the target range.
func GlucoseStatus(valueMmol, minMmol, maxMmol float64) string {
	if valueMmol < minMmol {
		return "low"
	}
	if valueMmol > maxMmol {
		return "high"
	}
	return "in_range"
}

//go:generate mockgen -destination=mocks/mock_note_repository.go -package=mocks . NoteRepository

// NoteRepository describes the storage for note entries.
type NoteRepository interface {
	Save(ctx context.Context, note *NoteEntry) error
	GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]*NoteEntry, error)
}
