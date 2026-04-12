//go:build e2e

package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:14-alpine",
		postgres.WithDatabase("shuggabuddy_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	// Apply migrations via goose.
	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	goose.SetBaseFS(nil)
	if err := goose.Up(sqlDB, "../../migrations"); err != nil {
		log.Fatalf("goose up: %v", err)
	}
	sqlDB.Close()

	// Create pgxpool for tests.
	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}

	code := m.Run()

	testPool.Close()
	if err := pgContainer.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminate container: %v\n", err)
	}
	os.Exit(code)
}
