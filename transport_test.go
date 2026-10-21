package catalogue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// stubService records which method was called and returns canned results.
type stubService struct {
	called   string
	calledID string
	stock    int
	err      error
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

func (s *stubService) Tags() ([]string, error) {
	s.called = "Tags"
	return tags, s.err
}

func (s *stubService) Health() []Health {
	s.called = "Health"
	return []Health{}
}

func serve(svc Service, path string) *httptest.ResponseRecorder {
	logger := log.NewLogfmtLogger(os.Stderr)
	tracer := stdopentracing.NoopTracer{}
	router := MakeHTTPHandler(context.Background(), MakeEndpoints(svc, tracer), "./images/", logger, tracer)
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", path, nil)
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
