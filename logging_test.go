package catalogue

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newStockAdjustmentCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "catalogue_stock_adjustments_total",
		Help: "Number of committed stock adjustments, by reason.",
	}, []string{"reason"})
}

func counterValues(t *testing.T, counter *prometheus.CounterVec) map[string]float64 {
	values := map[string]float64{}
	for _, reason := range StockReasons {
		var m dto.Metric
		if err := counter.WithLabelValues(reason).Write(&m); err != nil {
			t.Fatalf("reading counter for %s: %v", reason, err)
		}
		values[reason] = m.GetCounter().GetValue()
	}
	return values
}

func stockAdjustedLines(buf *bytes.Buffer) []string {
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "event=stock_adjusted") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestStockAuditMiddlewareInitialisesReasons(t *testing.T) {
	counter := newStockAdjustmentCounter()
	StockAuditMiddleware(log.NewNopLogger(), counter)

	registry := prometheus.NewRegistry()
	registry.MustRegister(counter)
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}
	have := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "catalogue_stock_adjustments_total" {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, label := range m.GetLabel() {
				if label.GetName() == "reason" {
					have[label.GetValue()] = m.GetCounter().GetValue()
				}
			}
		}
	}
	want := map[string]float64{"sale": 0, "return": 0, "restock": 0, "correction": 0}
	if len(have) != len(want) {
		t.Fatalf("want exported reasons %v, have %v", want, have)
	}
	for reason, value := range want {
		if v, ok := have[reason]; !ok || v != value {
			t.Errorf("reason %s: want exported with value %v, have %v (exported: %v)", reason, value, v, ok)
		}
	}
}

func TestStockAuditMiddlewareRecordsCommittedAdjustment(t *testing.T) {
	var buf bytes.Buffer
	counter := newStockAdjustmentCounter()
	svc := StockAuditMiddleware(log.NewLogfmtLogger(&buf), counter)(&stubService{stock: 12})

	n, err := svc.AdjustStock("X", 10, "restock")
	if err != nil || n != 12 {
		t.Fatalf("AdjustStock: want 12 and no error, have %d, %v", n, err)
	}

	want := map[string]float64{"sale": 0, "return": 0, "restock": 1, "correction": 0}
	if have := counterValues(t, counter); !mapsEqual(want, have) {
		t.Errorf("want counters %v, have %v", want, have)
	}

	lines := stockAdjustedLines(&buf)
	if len(lines) != 1 {
		t.Fatalf("want exactly one event=stock_adjusted line, have %d: %q", len(lines), buf.String())
	}
	for _, kv := range []string{"id=X", "delta=10", "reason=restock", "new_count=12"} {
		if !strings.Contains(lines[0], kv) {
			t.Errorf("want %s in log line %q", kv, lines[0])
		}
	}
}

func TestStockAuditMiddlewareIgnoresRejectedAdjustment(t *testing.T) {
	for _, rejection := range []error{ErrInsufficientStock, ErrInvalidReason, ErrInvalidDelta, ErrNotFound, ErrDBConnection} {
		var buf bytes.Buffer
		counter := newStockAdjustmentCounter()
		svc := StockAuditMiddleware(log.NewLogfmtLogger(&buf), counter)(&stubService{err: rejection})

		if _, err := svc.AdjustStock("X", -5, "sale"); err != rejection {
			t.Errorf("AdjustStock with %v: want the error passed through, have %v", rejection, err)
		}
		want := map[string]float64{"sale": 0, "return": 0, "restock": 0, "correction": 0}
		if have := counterValues(t, counter); !mapsEqual(want, have) {
			t.Errorf("AdjustStock with %v: want counters %v, have %v", rejection, want, have)
		}
		if lines := stockAdjustedLines(&buf); len(lines) != 0 {
			t.Errorf("AdjustStock with %v: want no event=stock_adjusted line, have %q", rejection, lines)
		}
	}
}

func TestStockAuditMiddlewarePassesOtherMethodsThrough(t *testing.T) {
	stub := &stubService{stock: 4}
	svc := StockAuditMiddleware(log.NewNopLogger(), newStockAdjustmentCounter())(stub)
	if n, err := svc.Stock("X"); n != 4 || err != nil || stub.called != "Stock" {
		t.Errorf("Stock(X): want 4 from the wrapped service, have %d, %v (called %s)", n, err, stub.called)
	}
}

func mapsEqual(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
