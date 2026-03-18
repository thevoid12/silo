// siloschema exposes the embedded SQLite schema DDL for runtime migrations.
package siloschema

import _ "embed"

//go:embed schema.sql
var SchemaSQL string
