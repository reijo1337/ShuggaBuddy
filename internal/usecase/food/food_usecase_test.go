package food_test

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
	"github.com/gmtantsevov/shuggabuddy/internal/usecase/food"
)

var now = time.Date(2025, 3, 10, 13, 45, 0, 0, time.UTC)

func TestSaveEntry_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, e *domain.FoodEntry) error {
			assert.InDelta(t, 60.0, e.CarbsGrams, 0.01)
			assert.Equal(t, int64(1), e.UserID)
			assert.Equal(t, "гречка", e.Note)
			assert.Equal(t, now, e.EatenAt)
			return nil
		})

	uc := food.New(repo)
	err := uc.SaveEntry(context.Background(), 1, 60.0, "гречка", now)
	require.NoError(t, err)
}

func TestSaveEntry_TooLow(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	uc := food.New(repo)
	err := uc.SaveEntry(context.Background(), 1, 0.0, "", now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveEntry_TooHigh(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	uc := food.New(repo)
	err := uc.SaveEntry(context.Background(), 1, 501.0, "", now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSaveEntry_BoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		carbs float64
	}{
		{"min", 0.1},
		{"max", 500.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockFoodRepository(ctrl)
			repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

			uc := food.New(repo)
			err := uc.SaveEntry(context.Background(), 1, tt.carbs, "", now)
			assert.NoError(t, err)
		})
	}
}

func TestSaveEntry_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	uc := food.New(repo)
	err := uc.SaveEntry(context.Background(), 1, 60.0, "", now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestGetLastEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	expected := []domain.FoodEntry{
		{ID: 1, UserID: 1, CarbsGrams: 60.0, EatenAt: now},
	}
	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(expected, nil)

	uc := food.New(repo)
	entries, err := uc.GetLastEntries(context.Background(), 1, 5)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.InDelta(t, 60.0, entries[0].CarbsGrams, 0.01)
}

func TestGetLastEntries_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockFoodRepository(ctrl)

	repo.EXPECT().GetLast(gomock.Any(), int64(1), 5).Return(nil, errors.New("db error"))

	uc := food.New(repo)
	_, err := uc.GetLastEntries(context.Background(), 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestFormatCarbs(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{60.0, "60"},
		{47.5, "47.5"},
		{100.0, "100"},
		{0.1, "0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, food.FormatCarbs(tt.input))
		})
	}
}
