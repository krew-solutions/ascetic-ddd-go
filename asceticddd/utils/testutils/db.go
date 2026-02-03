package testutils

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	pgsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pg"
)

func NewPgSessionPool() (session.SessionPool, error) {
	var db_username string = getEnv("DB_USERNAME", "devel")
	var db_password string = getEnv("DB_PASSWORD", "devel")
	var db_host string = getEnv("DB_HOST", "localhost")
	var db_port string = getEnv("DB_PORT", "5432")
	var db_basename string = getEnv("DB_DATABASE", "devel_grade")

	connString := "postgres://" + db_username + ":" + db_password + "@" + db_host + ":" + db_port + "/" + db_basename

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}

	return pgsession.NewSessionPool(pool), nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
