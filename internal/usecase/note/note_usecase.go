// Package note contains business logic for saving diary notes.
package note

import (
	"context"
	"fmt"
	"time"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// NoteUseCase is the interface for note business logic.
type NoteUseCase interface {
	SaveNote(ctx context.Context, userID int64, noteType domain.NoteType, wellbeing *domain.WellbeingValue, text *string) error
}

// UseCase implements NoteUseCase.
type UseCase struct {
	repo domain.NoteRepository
}

// New creates a new note UseCase.
func New(repo domain.NoteRepository) *UseCase {
	return &UseCase{repo: repo}
}

// SaveNote saves a diary note entry. Input validation is handled by the delivery layer.
func (uc *UseCase) SaveNote(ctx context.Context, userID int64, noteType domain.NoteType, wellbeing *domain.WellbeingValue, text *string) error {
	entry := &domain.NoteEntry{
		UserID:    userID,
		Type:      noteType,
		Wellbeing: wellbeing,
		Text:      text,
		CreatedAt: time.Now(),
	}

	if err := uc.repo.Save(ctx, entry); err != nil {
		return fmt.Errorf("note.SaveNote: %w", err)
	}

	return nil
}
