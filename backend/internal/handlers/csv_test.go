package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

func decodeCSVResult(t *testing.T, raw json.RawMessage) CSVResult {
	t.Helper()
	var res CSVResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal CSVResult: %v", err)
	}
	return res
}

func TestCSVHandler_Execute_MixedColumns(t *testing.T) {
	h := &CSVHandler{}
	result, err := h.Execute(context.Background(), json.RawMessage(
		`{"csv_data":"name,age\nAlice,30\nBob,25\nEve,not-a-number"}`,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	res := decodeCSVResult(t, result)
	if res.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", res.RowCount)
	}
	if res.ColumnCount != 2 {
		t.Errorf("ColumnCount = %d, want 2", res.ColumnCount)
	}
	if res.Columns[0].Numeric {
		t.Error("name column should be non-numeric")
	}
	if res.Columns[1].Numeric {
		t.Error("age column should be non-numeric once a non-numeric value is present")
	}
}

func TestCSVHandler_Execute_NumericColumn(t *testing.T) {
	h := &CSVHandler{}
	result, err := h.Execute(context.Background(), json.RawMessage(`{"csv_data":"score\n10\n20\n30"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	res := decodeCSVResult(t, result)
	col := res.Columns[0]
	if !col.Numeric {
		t.Fatal("score column should be numeric")
	}
	if col.Sum == nil || *col.Sum != 60 {
		t.Errorf("Sum = %v, want 60", col.Sum)
	}
	if col.Min == nil || *col.Min != 10 {
		t.Errorf("Min = %v, want 10", col.Min)
	}
	if col.Max == nil || *col.Max != 30 {
		t.Errorf("Max = %v, want 30", col.Max)
	}
	if col.Average == nil || *col.Average != 20 {
		t.Errorf("Average = %v, want 20", col.Average)
	}
}

func TestCSVHandler_Execute_MissingData(t *testing.T) {
	h := &CSVHandler{}
	if _, err := h.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("Execute() error = nil, want an error for missing csv_data")
	}
}

func TestCSVHandler_Execute_HeaderOnly(t *testing.T) {
	h := &CSVHandler{}
	result, err := h.Execute(context.Background(), json.RawMessage(`{"csv_data":"a,b,c"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res := decodeCSVResult(t, result); res.RowCount != 0 {
		t.Errorf("RowCount = %d, want 0", res.RowCount)
	}
}

func TestCSVHandler_Execute_MalformedCSV(t *testing.T) {
	h := &CSVHandler{}
	_, err := h.Execute(context.Background(), json.RawMessage(`{"csv_data":"a,b\n\"unterminated"}`))
	if err == nil {
		t.Fatal("Execute() error = nil, want a parse error for malformed CSV")
	}
}
