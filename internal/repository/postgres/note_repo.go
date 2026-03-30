package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type NoteRepository struct {
	pool *pgxpool.Pool
}

func NewNoteRepository(db *pgxpool.Pool) *NoteRepository {
	return &NoteRepository{pool: db}
}

func (r *NoteRepository) Save(ctx context.Context, note *domain.NoteEntry) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO notes (user_id, type, wellbeing, text, created_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		note.UserID, note.Type, note.Wellbeing, note.Text, note.CreatedAt,
	).Scan(&note.ID, &note.CreatedAt)
	if err != nil {
		return fmt.Errorf("NoteRepo.Save: %w", err)
	}
	return nil
}

func (r *NoteRepository) GetByTimeRange(ctx context.Context, userID int64, from, to time.Time) ([]*domain.NoteEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, type, wellbeing, text, created_at
		 FROM notes
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		 ORDER BY created_at ASC`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("NoteRepo.GetByTimeRange: %w", err)
	}
	defer rows.Close()

	var entries []*domain.NoteEntry
	for rows.Next() {
		e := &domain.NoteEntry{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.Type, &e.Wellbeing, &e.Text, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("NoteRepo.GetByTimeRange: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("NoteRepo.GetByTimeRange: rows: %w", err)
	}
	return entries, nil
}
