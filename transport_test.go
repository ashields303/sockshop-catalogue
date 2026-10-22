package catalogue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// stubService records which method was called and returns canned results.
type stubService struct {
	called    string
	calledID  string
	delta     int
	reason    string
	threshold int
	stock     int
	history   []StockAdjustment
	err       error
}

func (s *stubService) List(tags []string, order string, pageNum, pageSize int) ([]Sock, error) {
	s.called = "List"
	return []Sock{s1}, s.err
}

func (s *stubService) Count(tags []string) (int, error) {
	s.called = "Count"
	return 5, s.err
}

func (s *stubService) Get(id string) (Sock, error) {
	s.called, s.calledID = "Get", id
	return s1, s.err
}

func (s *stubService) Stock(id string) (int, error) {
	s.called, s.calledID = "Stock", id
	return s.stock, s.err
}

func (s *stubService) AdjustStock(id string, delta int, reason string) (int, error) {
	s.called, s.calledID, s.delta, s.reason = "AdjustStock", id, delta, reason
	return s.stock, s.err
}

func (s *stubService) StockHistory(id string) ([]StockAdjustment, error) {
	s.called, s.calledID = "StockHistory", id
	return s.history, s.err
}

func (s *stubService) LowStock(threshold int) ([]Sock, error) {
	s.called, s.threshold = "LowStock", threshold
	return []Sock{s1}, s.err
}

func (s *stubService) Tags() ([]string, error) {
	s.called = "Tags"
	return tags, s.err
}

func (s *stubService) Health() []Health {
	s.called = "Health"
	return []Health{}
}

func serve(svc Service, path string) *httptest.ResponseRecorder {
	return serveRequest(svc, "GET", path, "")
}

func serveRequest(svc Service, method, path, body string) *httptest.ResponseRecorder {
	logger := log.NewLogfmtLogger(os.Stderr)
	tracer := stdopentracing.NoopTracer{}
	router := MakeHTTPHandler(context.Background(), MakeEndpoints(svc, tracer), "./images/", logger, tracer)
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	router.ServeHTTP(rec, req)
	return rec
}

func TestTransportRouting(t *testing.T) {
	for _, testcase := range []struct {
		path   string
		method string
		id     string
	}{
		{"/catalogue", "List", ""},
		{"/catalogue/size", "Count", ""},
		{"/catalogue/3", "Get", "3"},
		{"/catalogue/3/stock", "Stock", "3"},
		{"/catalogue/3/stock/history", "StockHistory", "3"},
		{"/catalogue/low-stock?threshold=5", "LowStock", ""},
		{"/catalogue/low-stockx", "Get", "low-stockx"},
		{"/tags", "Tags", ""},
	} {
		svc := &stubService{stock: 3}
		rec := serve(svc, testcase.path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: want status %d, have %d", testcase.path, http.StatusOK, rec.Code)
		}
		if svc.called != testcase.method {
			t.Errorf("GET %s: want %s called, have %q", testcase.path, testcase.method, svc.called)
		}
		if svc.calledID != testcase.id {
			t.Errorf("GET %s: want id %q, have %q", testcase.path, testcase.id, svc.calledID)
		}
	}
}

func TestTransportStock(t *testing.T) {
	rec := serve(&stubService{stock: 3}, "/catalogue/X/stock")
	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d, have %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("want Content-Type application/json, have %q", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	want := map[string]interface{}{"id": "X", "stock": float64(3)}
	if len(body) != len(want) || body["id"] != want["id"] || body["stock"] != want["stock"] {
		t.Errorf("want body %v, have %v", want, body)
	}
}

func TestTransportStockErrors(t *testing.T) {
	for _, testcase := range []struct {
		err  error
		want int
	}{
		{ErrNotFound, http.StatusNotFound},
		{ErrDBConnection, http.StatusInternalServerError},
	} {
		rec := serve(&stubService{err: testcase.err}, "/catalogue/Y/stock")
		if rec.Code != testcase.want {
			t.Errorf("GET /catalogue/Y/stock with %v: want status %d, have %d", testcase.err, testcase.want, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("GET /catalogue/Y/stock with %v: want Content-Type application/json, have %q", testcase.err, ct)
		}
	}
}

// The stock route's error handling must not change the error behaviour of the
// existing get route. encodeError does not unwrap the go-kit transport error,
// so the get route answers 500 even for ErrNotFound. That is a known defect,
// left for a separate ticket.
func TestTransportGetErrorsUnchanged(t *testing.T) {
	for _, err := range []error{ErrNotFound, ErrDBConnection} {
		rec := serve(&stubService{err: err}, "/catalogue/Y")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("GET /catalogue/Y with %v: want status %d, have %d", err, http.StatusInternalServerError, rec.Code)
		}
	}
}

func TestTransportAdjustStock(t *testing.T) {
	svc := &stubService{stock: 7}
	rec := serveRequest(svc, "PATCH", "/catalogue/X/stock", `{"delta":-3,"reason":"sale"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d, have %d", http.StatusOK, rec.Code)
	}
	if svc.called != "AdjustStock" || svc.calledID != "X" || svc.delta != -3 || svc.reason != "sale" {
		t.Errorf("want AdjustStock(X, -3, sale), have %s(%s, %d, %s)", svc.called, svc.calledID, svc.delta, svc.reason)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("want Content-Type application/json, have %q", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	if len(body) != 2 || body["id"] != "X" || body["count"] != float64(7) {
		t.Errorf("want body {id: X, count: 7}, have %v", body)
	}
}

func TestTransportAdjustStockInvalidBody(t *testing.T) {
	for _, body := range []string{
		`{"delta":1,"reason":"gift"}`,
		`{"delta":1}`,
		`{"delta":0,"reason":"sale"}`,
		`{"reason":"sale"}`,
		`{"delta":1.5,"reason":"sale"}`,
		`{"delta":"3","reason":"sale"}`,
		`{"delta":1,"reason":5}`,
		`{"delta":-3,"reason":"sale"`,
		`[1]`,
		`not json`,
		``,
	} {
		svc := &stubService{}
		rec := serveRequest(svc, "PATCH", "/catalogue/X/stock", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PATCH with body %s: want status %d, have %d", body, http.StatusBadRequest, rec.Code)
		}
		if svc.called != "" {
			t.Errorf("PATCH with body %s: want service not called, have %s", body, svc.called)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("PATCH with body %s: want Content-Type application/json, have %q", body, ct)
		}
	}
}

func TestTransportAdjustStockErrors(t *testing.T) {
	for _, testcase := range []struct {
		err  error
		want int
	}{
		{ErrNotFound, http.StatusNotFound},
		{ErrInsufficientStock, http.StatusConflict},
		{ErrInvalidReason, http.StatusBadRequest},
		{ErrInvalidDelta, http.StatusBadRequest},
		{ErrDBConnection, http.StatusInternalServerError},
	} {
		rec := serveRequest(&stubService{err: testcase.err}, "PATCH", "/catalogue/Y/stock", `{"delta":-1,"reason":"sale"}`)
		if rec.Code != testcase.want {
			t.Errorf("PATCH /catalogue/Y/stock with %v: want status %d, have %d", testcase.err, testcase.want, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("PATCH /catalogue/Y/stock with %v: want Content-Type application/json, have %q", testcase.err, ct)
		}
	}
}

// A run of rejected adjustments must not open the circuit breaker and block
// the next valid adjustment.
func TestTransportAdjustStockConflictsDoNotTripBreaker(t *testing.T) {
	svc := &stubService{err: ErrInsufficientStock}
	tracer := stdopentracing.NoopTracer{}
	router := MakeHTTPHandler(context.Background(), MakeEndpoints(svc, tracer), "./images/", log.NewNopLogger(), tracer)
	patch := func() int {
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest("PATCH", "/catalogue/X/stock", strings.NewReader(`{"delta":-1,"reason":"sale"}`))
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 10; i++ {
		if code := patch(); code != http.StatusConflict {
			t.Fatalf("rejected adjustment %d: want status %d, have %d", i+1, http.StatusConflict, code)
		}
	}
	svc.err, svc.stock = nil, 1
	if code := patch(); code != http.StatusOK {
		t.Errorf("valid adjustment after conflicts: want status %d, have %d", http.StatusOK, code)
	}
}

func TestTransportStockHistory(t *testing.T) {
	svc := &stubService{history: []StockAdjustment{
		{Delta: -3, Reason: "sale", CreatedAt: time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)},
		{Delta: 10, Reason: "restock", CreatedAt: time.Date(2026, 10, 21, 17, 5, 9, 0, time.UTC)},
	}}
	rec := serve(svc, "/catalogue/X/stock/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d, have %d", http.StatusOK, rec.Code)
	}
	if svc.calledID != "X" {
		t.Errorf("want StockHistory(X), have StockHistory(%s)", svc.calledID)
	}
	want := `[{"delta":-3,"reason":"sale","timestamp":"2026-10-22T09:30:00Z"},{"delta":10,"reason":"restock","timestamp":"2026-10-21T17:05:09Z"}]`
	if have := strings.TrimSpace(rec.Body.String()); have != want {
		t.Errorf("want body %s, have %s", want, have)
	}

	rec = serve(&stubService{history: []StockAdjustment{}}, "/catalogue/X/stock/history")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("no adjustments: want 200 [], have %d %s", rec.Code, rec.Body.String())
	}

	for _, testcase := range []struct {
		err  error
		want int
	}{
		{ErrNotFound, http.StatusNotFound},
		{ErrDBConnection, http.StatusInternalServerError},
	} {
		rec := serve(&stubService{err: testcase.err}, "/catalogue/Y/stock/history")
		if rec.Code != testcase.want {
			t.Errorf("GET /catalogue/Y/stock/history with %v: want status %d, have %d", testcase.err, testcase.want, rec.Code)
		}
	}
}

func TestTransportLowStock(t *testing.T) {
	svc := &stubService{}
	rec := serve(svc, "/catalogue/low-stock?threshold=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d, have %d", http.StatusOK, rec.Code)
	}
	if svc.called != "LowStock" || svc.threshold != 5 {
		t.Errorf("want LowStock(5), have %s(%d)", svc.called, svc.threshold)
	}
	var body []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("want a JSON array, have %q: %v", rec.Body.String(), err)
	}
	if len(body) != 1 || body[0]["id"] != s1.ID {
		t.Errorf("want [%s], have %v", s1.ID, body)
	}

	svc = &stubService{}
	if rec := serve(svc, "/catalogue/low-stock?threshold=0"); rec.Code != http.StatusOK || svc.threshold != 0 {
		t.Errorf("threshold=0: want 200 and LowStock(0), have %d and %s(%d)", rec.Code, svc.called, svc.threshold)
	}

	for _, query := range []string{"", "?threshold=", "?threshold=-1", "?threshold=abc", "?threshold=1.5", "?size=5"} {
		svc := &stubService{}
		rec := serve(svc, "/catalogue/low-stock"+query)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET /catalogue/low-stock%s: want status %d, have %d", query, http.StatusBadRequest, rec.Code)
		}
		if svc.called != "" {
			t.Errorf("GET /catalogue/low-stock%s: want no service call, have %s", query, svc.called)
		}
	}
}
