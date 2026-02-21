package db

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect() error {
	var err error
	Pool, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	return err
}
