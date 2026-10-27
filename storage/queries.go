package storage

import (
	"embed"
	"fmt"
	"strings"
)

// SQL lives in storage/sql and is compiled into the binary. Named blocks keep
// domain files readable while every call site still gets one statement it can
// pass to database/sql directly.
//
//go:embed sql/*.sql
var sqlFiles embed.FS

var schemaSQL = mustSQLFile("sql/schema.sql")

var sqlQueries = mustLoadQueries()

func mustSQLFile(name string) string {
	data, err := sqlFiles.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("storage: cannot read embedded %s: %v", name, err))
	}
	return string(data)
}

func mustLoadQueries() map[string]string {
	entries, err := sqlFiles.ReadDir("sql")
	if err != nil {
		panic(fmt.Sprintf("storage: cannot list embedded SQL: %v", err))
	}
	queries := make(map[string]string, len(entries)*4)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "schema.sql" {
			continue
		}
		lines := strings.Split(mustSQLFile("sql/"+name), "\n")
		current := ""
		var body strings.Builder
		flush := func() {
			if current == "" {
				return
			}
			query := strings.TrimSpace(body.String())
			if query == "" {
				panic(fmt.Sprintf("storage: empty SQL query %q in %s", current, name))
			}
			if _, exists := queries[current]; exists {
				panic(fmt.Sprintf("storage: duplicate SQL query %q", current))
			}
			queries[current] = query
			body.Reset()
		}
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "-- name:") {
				flush()
				current = strings.TrimSpace(strings.TrimPrefix(trimmed, "-- name:"))
				if current == "" {
					panic(fmt.Sprintf("storage: empty SQL query name in %s", name))
				}
				continue
			}
			if current != "" {
				body.WriteString(line)
				body.WriteByte('\n')
			}
		}
		flush()
	}
	return queries
}

func sqlQuery(name string) string {
	query, ok := sqlQueries[name]
	if !ok {
		panic("storage: unknown SQL query " + name)
	}
	return query
}
