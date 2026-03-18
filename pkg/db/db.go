// Open opens a SQLite database at path and runs schema migrations.
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	siloerrors "silo/pkg/errors"
	siloschema "silo/pkg/db/sqlc/silo"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", siloerrors.ErrDBOpen, err)
	}
	database.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := database.Exec(siloschema.SchemaSQL); err != nil {
		database.Close()
		return nil, fmt.Errorf("%w: %s", siloerrors.ErrDBMigrate, err)
	}
	return database, nil
}
