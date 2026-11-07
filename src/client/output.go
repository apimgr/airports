package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

func printTableResult(body []byte) error {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("table output requires a JSON response: %w", err)
	}
	rows := tableRows(value)
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No results.")
		return nil
	}
	columns := tableColumns(rows)
	var output bytes.Buffer
	writer := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	for index, column := range columns {
		if index > 0 {
			_, _ = fmt.Fprint(writer, "\t")
		}
		_, _ = fmt.Fprint(writer, strings.ToUpper(column))
	}
	_, _ = fmt.Fprintln(writer)
	for _, row := range rows {
		for index, column := range columns {
			if index > 0 {
				_, _ = fmt.Fprint(writer, "\t")
			}
			_, _ = fmt.Fprint(writer, tableCell(row[column]))
		}
		_, _ = fmt.Fprintln(writer)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("format table output: %w", err)
	}
	fmt.Print(output.String())
	return nil
}

func printCSVResult(body []byte) error {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("CSV output requires a JSON response: %w", err)
	}
	rows := tableRows(value)
	if len(rows) == 0 {
		return nil
	}
	columns := tableColumns(rows)
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write(columns); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for index, column := range columns {
			record[index] = tableCell(row[column])
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("format CSV output: %w", err)
	}
	fmt.Print(output.String())
	return nil
}

func tableRows(value any) []map[string]any {
	if rows, ok := value.([]any); ok {
		return toTableRows(rows)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"data", "results", "airports"} {
		if rows, ok := object[key].([]any); ok {
			return toTableRows(rows)
		}
	}
	return []map[string]any{object}
}

func toTableRows(values []any) []map[string]any {
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if row, ok := value.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func tableColumns(rows []map[string]any) []string {
	columns := make([]string, 0)
	seen := make(map[string]struct{})
	for _, row := range rows {
		for column := range row {
			if _, ok := seen[column]; !ok {
				seen[column] = struct{}{}
				columns = append(columns, column)
			}
		}
	}
	preferred := []string{"icao", "iata", "name", "city", "state", "country", "status", "version", "latitude", "longitude", "lat", "lon"}
	sort.SliceStable(columns, func(i, j int) bool {
		left, right := tableColumnOrder(columns[i], preferred), tableColumnOrder(columns[j], preferred)
		if left != right {
			return left < right
		}
		return columns[i] < columns[j]
	})
	return columns
}

func tableColumnOrder(column string, preferred []string) int {
	for index, value := range preferred {
		if column == value {
			return index
		}
	}
	return len(preferred)
}

func tableCell(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "\t", " ")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}
