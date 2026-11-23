package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type ownerResourceQuery interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

type ownerResourceExecutor interface {
	ownerResourceQuery
	Exec(string, ...any) (sql.Result, error)
}

// OwnerResourceTables finds non-system tables that belong to an owner.
func OwnerResourceTables(query ownerResourceQuery) ([]string, error) {
	tables, err := query.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('owners','api_tokens')`)
	if err != nil {
		return nil, err
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			_ = tables.Close()
			return nil, err
		}
		names = append(names, name)
	}
	if err := tables.Err(); err != nil {
		_ = tables.Close()
		return nil, err
	}
	if err := tables.Close(); err != nil {
		return nil, err
	}
	var owned []string
	for _, name := range names {
		columns, err := query.Query(`PRAGMA table_info("` + strings.ReplaceAll(name, `"`, `""`) + `")`)
		if err != nil {
			return nil, err
		}
		foundOwnerColumn := false
		for columns.Next() {
			var cid, notNull, primaryKey int
			var column, kind string
			var defaultValue any
			if err := columns.Scan(&cid, &column, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
				_ = columns.Close()
				return nil, err
			}
			if column == "owner_id" {
				foundOwnerColumn = true
			}
		}
		if err := columns.Err(); err != nil {
			_ = columns.Close()
			return nil, err
		}
		if err := columns.Close(); err != nil {
			return nil, err
		}
		if foundOwnerColumn {
			owned = append(owned, name)
		}
	}
	return owned, nil
}

// DeleteOwnerResources removes one owner's rows from every owner resource table.
func DeleteOwnerResources(tx ownerResourceExecutor, owner string) (int64, error) {
	tables, err := OwnerResourceTables(tx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, table := range tables {
		result, err := tx.Exec(`DELETE FROM "`+strings.ReplaceAll(table, `"`, `""`)+`" WHERE owner_id = ?`, owner)
		if err != nil {
			return 0, fmt.Errorf("delete owner resources from %s: %w", table, err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// CountOwnerResources counts one owner's rows across all owner resource tables.
func CountOwnerResources(query ownerResourceQuery, owner string) (int64, error) {
	tables, err := OwnerResourceTables(query)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, table := range tables {
		var count int64
		if err := query.QueryRow(`SELECT COUNT(*) FROM "`+strings.ReplaceAll(table, `"`, `""`)+`" WHERE owner_id = ?`, owner).Scan(&count); err != nil {
			return 0, fmt.Errorf("count owner resources in %s: %w", table, err)
		}
		total += count
	}
	return total, nil
}
