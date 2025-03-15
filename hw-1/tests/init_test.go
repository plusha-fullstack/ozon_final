package postgresql

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
)

type TDB struct {
	DB *pgxpool.Pool
}

func NewFromEnv() *TDB {

	connString := "postgres://postgres:postgres@localhost:5432/test?sslmode=disable"

	pool, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	return &TDB{DB: pool}
}

func (tdb *TDB) SetUp(t *testing.T, fixture string) {
	t.Helper()

	_, err := tdb.DB.Exec(context.Background(), "TRUNCATE orders, returns, order_history CASCADE")
	require.NoError(t, err)

	if fixture != "" {
		fixtureSQL, err := os.ReadFile(fmt.Sprintf("../fixtures/%s.sql", fixture))
		if err == nil {
			_, err = tdb.DB.Exec(context.Background(), string(fixtureSQL))
			require.NoError(t, err)
		}
	}
}

func (tdb *TDB) TearDown(t *testing.T) {
	t.Helper()

	_, err := tdb.DB.Exec(context.Background(), "TRUNCATE orders, returns, order_history CASCADE")
	require.NoError(t, err)
}
