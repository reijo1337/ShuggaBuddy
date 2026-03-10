package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

type ExternalAccountRepo struct {
	pool *pgxpool.Pool
}

func NewExternalAccountRepo(pool *pgxpool.Pool) *ExternalAccountRepo {
	return &ExternalAccountRepo{pool: pool}
}

func (r *ExternalAccountRepo) GetByProvider(ctx context.Context, provider domain.Provider, externalID string) (*domain.ExternalAccount, error) {
	acc := &domain.ExternalAccount{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, external_id, display_name, created_at
		 FROM external_accounts
		 WHERE provider = $1 AND external_id = $2`,
		provider, externalID,
	).Scan(&acc.ID, &acc.UserID, &acc.Provider, &acc.ExternalID, &acc.DisplayName, &acc.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ExternalAccountRepo.GetByProvider: %w", err)
	}

	return acc, nil
}

func (r *ExternalAccountRepo) Create(ctx context.Context, account *domain.ExternalAccount) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO external_accounts (user_id, provider, external_id, display_name)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		account.UserID, account.Provider, account.ExternalID, account.DisplayName,
	).Scan(&account.ID)
	if err != nil {
		return fmt.Errorf("ExternalAccountRepo.Create: %w", err)
	}

	return nil
}
