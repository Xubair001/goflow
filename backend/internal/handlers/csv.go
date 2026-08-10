package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// CSVJobType is the job.Registry key for CSVHandler.
const CSVJobType = "process_csv"

// CSVPayload is the process_csv job's input: raw CSV text with a header row.
type CSVPayload struct {
	Data string `json:"csv_data"`
}

// ColumnSummary describes one CSV column. Sum/Min/Max/Average are nil for
// non-numeric columns (or a column with no parseable values) so "no
// meaningful value" is distinguishable in JSON from "the value is zero".
type ColumnSummary struct {
	Name    string   `json:"name"`
	Numeric bool     `json:"numeric"`
	Count   int      `json:"count"`
	Sum     *float64 `json:"sum,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Average *float64 `json:"average,omitempty"`
}

// CSVResult is the process_csv job's output.
type CSVResult struct {
	RowCount    int             `json:"row_count"`
	ColumnCount int             `json:"column_count"`
	Columns     []ColumnSummary `json:"columns"`
}

// CSVHandler parses CSV text and summarizes each column: row/column counts,
// and for columns where every value parses as a number, min/max/sum/average.
type CSVHandler struct{}

var _ job.Handler = (*CSVHandler)(nil)

// Execute implements job.Handler.
func (h *CSVHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var p CSVPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode csv payload: %w", err)
	}
	if strings.TrimSpace(p.Data) == "" {
		return nil, errors.New("handlers: csv payload missing \"csv_data\"")
	}

	records, err := csv.NewReader(strings.NewReader(p.Data)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("handlers: parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("handlers: csv has no rows")
	}

	header, rows := records[0], records[1:]
	columns := make([]ColumnSummary, len(header))
	for i, name := range header {
		columns[i] = summarizeColumn(name, rows, i)
	}

	return json.Marshal(CSVResult{
		RowCount:    len(rows),
		ColumnCount: len(header),
		Columns:     columns,
	})
}

func summarizeColumn(name string, rows [][]string, col int) ColumnSummary {
	values := make([]float64, 0, len(rows))
	numeric := true
	for _, row := range rows {
		if col >= len(row) {
			numeric = false
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(row[col]), 64)
		if err != nil {
			numeric = false
			continue
		}
		values = append(values, v)
	}
	if !numeric || len(values) == 0 {
		return ColumnSummary{Name: name, Numeric: false, Count: len(rows)}
	}

	sum, min, max := 0.0, values[0], values[0]
	for _, v := range values {
		sum += v
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
	avg := sum / float64(len(values))
	return ColumnSummary{
		Name: name, Numeric: true, Count: len(values),
		Sum: &sum, Min: &min, Max: &max, Average: &avg,
	}
}
