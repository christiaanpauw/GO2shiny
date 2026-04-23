package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/christiaanpauw/GO2shiny/internal/db"
	"github.com/christiaanpauw/GO2shiny/internal/handlers"
)

// mockTableQuerier is a test double for db.TableQuerier.
type mockTableQuerier struct {
	page db.TablePage
	err  error
}

func (m *mockTableQuerier) GetTablePage(_ context.Context, page, size int, q, typeIE, typeGS string, yearFrom, yearTo int) (db.TablePage, error) {
	if m.err != nil {
		return db.TablePage{}, m.err
	}
	result := m.page
	// Filter rows by q if set (simple substring for mock purposes).
	if q != "" {
		var filtered []db.TableRow
		for _, r := range m.page.Rows {
			if contains(r.Country, q) || contains(r.TypeIE, q) ||
				contains(r.TypeGS, q) || contains(r.Commodity, q) {
				filtered = append(filtered, r)
			}
		}
		result.Rows = filtered
		result.Total = len(filtered)
	}
	result.Page = page
	result.Size = size
	return result, nil
}

// contains is a case-insensitive substring check helper for the mock.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		stringContainsFold(s, sub))
}

func stringContainsFold(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// sampleRows returns a set of test rows including a dairy entry.
func sampleRows() []db.TableRow {
	return []db.TableRow{
		{Year: 2023, Country: "China", TypeIE: "Exports", TypeGS: "Goods", Commodity: "Dairy", ValueNZD: 7.1},
		{Year: 2023, Country: "Australia", TypeIE: "Imports", TypeGS: "Services", Commodity: "Tourism", ValueNZD: 3.2},
		{Year: 2022, Country: "USA", TypeIE: "Exports", TypeGS: "Goods", Commodity: "Meat", ValueNZD: 4.5},
	}
}

// TestTableAPIDefaults verifies that default pagination (page=1, size=25)
// returns correct JSON with the expected fields.
func TestTableAPIDefaults(t *testing.T) {
	rows := sampleRows()
	mock := &mockTableQuerier{
		page: db.TablePage{
			Total: len(rows),
			Page:  1,
			Size:  25,
			Rows:  rows,
		},
	}

	h := handlers.TableAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/table", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var resp db.TablePage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Page != 1 {
		t.Errorf("want page=1, got %d", resp.Page)
	}
	if resp.Size != 25 {
		t.Errorf("want size=25, got %d", resp.Size)
	}
	if resp.Total != len(rows) {
		t.Errorf("want total=%d, got %d", len(rows), resp.Total)
	}
	if len(resp.Rows) == 0 {
		t.Error("want non-empty rows")
	}
}

// TestTableAPISearch verifies that ?q=dairy filters results correctly.
func TestTableAPISearch(t *testing.T) {
	mock := &mockTableQuerier{
		page: db.TablePage{
			Total: len(sampleRows()),
			Rows:  sampleRows(),
		},
	}

	h := handlers.TableAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/table?q=dairy", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp db.TablePage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("want 1 row matching 'dairy', got %d", resp.Total)
	}
	if len(resp.Rows) != 1 || resp.Rows[0].Commodity != "Dairy" {
		t.Errorf("expected only Dairy row, got %+v", resp.Rows)
	}
}

// TestTableAPIMaxPageSize verifies that requesting size=9999 is capped at 100.
func TestTableAPIMaxPageSize(t *testing.T) {
	mock := &mockTableQuerier{
		page: db.TablePage{Total: 0, Rows: []db.TableRow{}},
	}

	h := handlers.TableAPIHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/table?size=9999", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp db.TablePage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Size != 100 {
		t.Errorf("want size capped at 100, got %d", resp.Size)
	}
}

// TestTableAPIInvalidPage verifies that ?page=abc returns 400 Bad Request.
func TestTableAPIInvalidPage(t *testing.T) {
	mock := &mockTableQuerier{}
	h := handlers.TableAPIHandler(mock)

	cases := []struct {
		name string
		url  string
	}{
		{"non-numeric page", "/api/trade/table?page=abc"},
		{"zero page", "/api/trade/table?page=0"},
		{"negative page", "/api/trade/table?page=-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			h(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", w.Code)
			}
		})
	}
}

// TestTableAPINilQuerier verifies that a nil querier returns 503.
func TestTableAPINilQuerier(t *testing.T) {
	h := handlers.TableAPIHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/trade/table", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
