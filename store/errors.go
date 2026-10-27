package store

import "errors"

// ErrCorruptDatabase means the file is not an acceptable yoanime database:
// it either misses a table of the current schema or a column of an existing
// table. Old releases are intentionally unsupported.
var ErrCorruptDatabase = errors.New("database is corrupted or was created by another version")
