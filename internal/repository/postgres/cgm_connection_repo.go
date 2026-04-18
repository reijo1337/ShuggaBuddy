package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type CGMConnectionRepo struct {
	pool *pgxpool.Pool
}

func NewCGMConnectionRepo(pool *pgxpool.Pool) *CGMConnectionRepo {
	return &CGMConnectionRepo{pool: pool}
}

func (r *CGMConnectionRepo) Upsert(ctx context.Context, conn *domain.CGMConnection) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cgm_connections (user_id, provider, base_url, api_token, active)
		 VALUES ($1, $2, $3, $4, TRUE)
		 ON CONFLICT (user_id, provider) DO UPDATE
		 SET base_url = EXCLUDED.base_url,
		     api_token = EXCLUDED.api_token,
		     active = TRUE,
		     last_synced_at = NULL
		 RETURNING id, created_at`,
		conn.UserID, conn.Provider, conn.BaseURL, conn.APIToken,
	).Scan(&conn.ID, &conn.CreatedAt)
	if err != nil {
		return fmt.Errorf("CGMConnectionRepo.Upsert: %w", err)
	}
	return nil
}

func (r *CGMConnectionRepo) GetByUserID(ctx context.Context, userID int64) (*domain.CGMConnection, error) {
	conn := &domain.CGMConnection{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, base_url, api_token, last_synced_at, active, created_at
		 FROM cgm_connections
		 WHERE user_id = $1`,
		userID,
	).Scan(&conn.ID, &conn.UserID, &conn.Provider, &conn.BaseURL, &conn.APIToken,
		&conn.LastSyncedAt, &conn.Active, &conn.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("CGMConnectionRepo.GetByUserID: %w", err)
	}
	return conn, nil
}

func (r *CGMConnectionRepo) GetAllActive(ctx context.Context) ([]domain.CGMConnection, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, provider, base_url, api_token, last_synced_at, active, created_at
		 FROM cgm_connections
		 WHERE active = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("CGMConnectionRepo.GetAllActive: %w", err)
	}
	defer rows.Close()

	var conns []domain.CGMConnection
	for rows.Next() {
		var c domain.CGMConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Provider, &c.BaseURL, &c.APIToken,
			&c.LastSyncedAt, &c.Active, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("CGMConnectionRepo.GetAllActive: scan: %w", err)
		}
		conns = append(conns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CGMConnectionRepo.GetAllActive: rows: %w", err)
	}
	return conns, nil
}

func (r *CGMConnectionRepo) UpdateLastSyncedAt(ctx context.Context, id int64, syncedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE cgm_connections SET last_synced_at = $1 WHERE id = $2`,
		syncedAt, id,
	)
	if err != nil {
		return fmt.Errorf("CGMConnectionRepo.UpdateLastSyncedAt: %w", err)
	}
	return nil
}

func (r *CGMConnectionRepo) Deactivate(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE cgm_connections SET active = FALSE WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("CGMConnectionRepo.Deactivate: %w", err)
	}
	return nil
}

func (r *CGMConnectionRepo) Delete(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM cgm_connections WHERE user_id = $1`, userID,
	)
	if err != nil {
		return fmt.Errorf("CGMConnectionRepo.Delete: %w", err)
	}
	return nil
}
