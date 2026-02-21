package pg

import (
	"github.com/jackc/pgx/v5"
)

// rowsAdapter adapts pgx.Rows to session.Rows
type rowsAdapter struct {
	rows pgx.Rows
}

func (r *rowsAdapter) Close() error {
	r.rows.Close()
	return nil
}

func (r *rowsAdapter) Err() error {
	return r.rows.Err()
}

func (r *rowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r *rowsAdapter) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

// rowAdapter adapts pgx.Row to session.Row
type rowAdapter struct {
	row pgx.Row
	err error
}

func (r *rowAdapter) Err() error {
	return r.err
}

func (r *rowAdapter) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if r.err == nil {
		r.err = err
	}
	return err
}

type errorRow struct {
	err error
}

func (r *errorRow) Err() error {
	return r.err
}

func (r *errorRow) Scan(...any) error {
	return r.err
}
