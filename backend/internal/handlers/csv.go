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

// CSVPayload is the process_csv job's input: raw CSV text with a header
// row. When EmailTo is set, the computed summary is also emailed there via
// Mailer -- e.g. "analyze this data and send me the results."
type CSVPayload struct {
	Data    string `json:"csv_data"`
	EmailTo string `json:"email_to,omitempty"`
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

// CSVResult is the process_csv job's output. EmailedTo is set only when the
// payload requested delivery and it succeeded.
type CSVResult struct {
	RowCount    int             `json:"row_count"`
	ColumnCount int             `json:"column_count"`
	Columns     []ColumnSummary `json:"columns"`
	EmailedTo   string          `json:"emailed_to,omitempty"`
}

// CSVHandler parses CSV text and summarizes each column: row/column counts,
// and for columns where every value parses as a number, min/max/sum/average.
// Mailer is optional -- leave it nil if send_email isn't configured; an
// EmailTo request without one fails with a clear error instead of silently
// no-op'ing.
type CSVHandler struct {
	Mailer *Mailer
}

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

	result := CSVResult{
		RowCount:    len(rows),
		ColumnCount: len(header),
		Columns:     columns,
	}

	if p.EmailTo != "" {
		if h.Mailer == nil {
			return nil, errors.New("handlers: csv analysis requested email delivery but no SMTP mailer is configured")
		}
		if err := h.Mailer.Send(p.EmailTo, "GoFlow CSV analysis", csvEmailBody(result)); err != nil {
			return nil, fmt.Errorf("handlers: email csv analysis: %w", err)
		}
		result.EmailedTo = p.EmailTo
	}

	return json.Marshal(result)
}

func csvEmailBody(r CSVResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d rows, %d columns\n\n", r.RowCount, r.ColumnCount)
	for _, c := range r.Columns {
		if c.Numeric {
			fmt.Fprintf(&b, "%s: count=%d sum=%.2f min=%.2f max=%.2f average=%.2f\n",
				c.Name, c.Count, *c.Sum, *c.Min, *c.Max, *c.Average)
		} else {
			fmt.Fprintf(&b, "%s: count=%d (non-numeric)\n", c.Name, c.Count)
		}
	}
	return b.String()
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
