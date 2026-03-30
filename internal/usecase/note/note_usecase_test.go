package note_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/note"
)

func TestSaveNote_Wellbeing(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNoteRepository(ctrl)

	wellbeing := domain.WellbeingGood

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, e *domain.NoteEntry) error {
			assert.Equal(t, int64(1), e.UserID)
			assert.Equal(t, domain.NoteTypeWellbeing, e.Type)
			require.NotNil(t, e.Wellbeing)
			assert.Equal(t, domain.WellbeingGood, *e.Wellbeing)
			assert.Nil(t, e.Text)
			assert.False(t, e.CreatedAt.IsZero())
			return nil
		})

	uc := note.New(repo)
	err := uc.SaveNote(context.Background(), 1, domain.NoteTypeWellbeing, &wellbeing, nil)
	require.NoError(t, err)
}

func TestSaveNote_FreeText(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNoteRepository(ctrl)

	text := "feeling stressed"

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, e *domain.NoteEntry) error {
			assert.Equal(t, int64(2), e.UserID)
			assert.Equal(t, domain.NoteTypeFree, e.Type)
			assert.Nil(t, e.Wellbeing)
			require.NotNil(t, e.Text)
			assert.Equal(t, "feeling stressed", *e.Text)
			assert.False(t, e.CreatedAt.IsZero())
			return nil
		})

	uc := note.New(repo)
	err := uc.SaveNote(context.Background(), 2, domain.NoteTypeFree, nil, &text)
	require.NoError(t, err)
}

func TestSaveNote_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNoteRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(errors.New("db error"))

	uc := note.New(repo)
	err := uc.SaveNote(context.Background(), 1, domain.NoteTypeStress, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}
