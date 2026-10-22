package catalogue

import (
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

var (
	s1 = Sock{ID: "1", Name: "name1", Description: "description1", Price: 1.1, Count: 1, ImageURL: []string{"ImageUrl_11", "ImageUrl_21"}, ImageURL_1: "ImageUrl_11", ImageURL_2: "ImageUrl_21", Tags: []string{"odd", "prime"}, TagString: "odd,prime"}
	s2 = Sock{ID: "2", Name: "name2", Description: "description2", Price: 1.2, Count: 2, ImageURL: []string{"ImageUrl_12", "ImageUrl_22"}, ImageURL_1: "ImageUrl_12", ImageURL_2: "ImageUrl_22", Tags: []string{"even", "prime"}, TagString: "even,prime"}
	s3 = Sock{ID: "3", Name: "name3", Description: "description3", Price: 1.3, Count: 3, ImageURL: []string{"ImageUrl_13", "ImageUrl_23"}, ImageURL_1: "ImageUrl_13", ImageURL_2: "ImageUrl_23", Tags: []string{"odd", "prime"}, TagString: "odd,prime"}
	s4 = Sock{ID: "4", Name: "name4", Description: "description4", Price: 1.4, Count: 4, ImageURL: []string{"ImageUrl_14", "ImageUrl_24"}, ImageURL_1: "ImageUrl_14", ImageURL_2: "ImageUrl_24", Tags: []string{"even"}, TagString: "even"}
	s5 = Sock{ID: "5", Name: "name5", Description: "description5", Price: 1.5, Count: 5, ImageURL: []string{"ImageUrl_15", "ImageUrl_25"}, ImageURL_1: "ImageUrl_15", ImageURL_2: "ImageUrl_25", Tags: []string{"odd", "prime"}, TagString: "odd,prime"}

	socks = []Sock{s1, s2, s3, s4, s5}
	tags  = []string{"odd", "even", "prime"}
)

var logger log.Logger

func TestCatalogueServiceList(t *testing.T) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	var cols []string = []string{"id", "name", "description", "price", "count", "image_url_1", "image_url_2", "tag_name"}

	// Test Case 1
	mock.ExpectQuery("SELECT *").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(s1.ID, s1.Name, s1.Description, s1.Price, s1.Count, s1.ImageURL[0], s1.ImageURL[1], strings.Join(s1.Tags, ",")).
		AddRow(s2.ID, s2.Name, s2.Description, s2.Price, s2.Count, s2.ImageURL[0], s2.ImageURL[1], strings.Join(s2.Tags, ",")).
		AddRow(s3.ID, s3.Name, s3.Description, s3.Price, s3.Count, s3.ImageURL[0], s3.ImageURL[1], strings.Join(s3.Tags, ",")).
		AddRow(s4.ID, s4.Name, s4.Description, s4.Price, s4.Count, s4.ImageURL[0], s4.ImageURL[1], strings.Join(s4.Tags, ",")).
		AddRow(s5.ID, s5.Name, s5.Description, s5.Price, s5.Count, s5.ImageURL[0], s5.ImageURL[1], strings.Join(s5.Tags, ",")))

	// Test Case 2
	mock.ExpectQuery("SELECT *").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(s4.ID, s4.Name, s4.Description, s4.Price, s4.Count, s4.ImageURL[0], s4.ImageURL[1], strings.Join(s4.Tags, ",")).
		AddRow(s1.ID, s1.Name, s1.Description, s1.Price, s1.Count, s1.ImageURL[0], s1.ImageURL[1], strings.Join(s1.Tags, ",")).
		AddRow(s2.ID, s2.Name, s2.Description, s2.Price, s2.Count, s2.ImageURL[0], s2.ImageURL[1], strings.Join(s2.Tags, ",")))

	// // Test Case 3
	mock.ExpectQuery("SELECT *").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(s1.ID, s1.Name, s1.Description, s1.Price, s1.Count, s1.ImageURL[0], s1.ImageURL[1], strings.Join(s1.Tags, ",")).
		AddRow(s3.ID, s3.Name, s3.Description, s3.Price, s3.Count, s3.ImageURL[0], s3.ImageURL[1], strings.Join(s3.Tags, ",")).
		AddRow(s5.ID, s5.Name, s5.Description, s5.Price, s5.Count, s5.ImageURL[0], s5.ImageURL[1], strings.Join(s5.Tags, ",")))

	s := NewCatalogueService(sqlxDB, logger)
	for _, testcase := range []struct {
		tags     []string
		order    string
		pageNum  int
		pageSize int
		want     []Sock
	}{
		{
			tags:     []string{},
			order:    "",
			pageNum:  1,
			pageSize: 5,
			want:     []Sock{s1, s2, s3, s4, s5},
		},
		{
			tags:     []string{},
			order:    "tag",
			pageNum:  1,
			pageSize: 3,
			want:     []Sock{s4, s1, s2},
		},
		{
			tags:     []string{"odd"},
			order:    "id",
			pageNum:  2,
			pageSize: 2,
			want:     []Sock{s5},
		},
	} {
		have, err := s.List(testcase.tags, testcase.order, testcase.pageNum, testcase.pageSize)
		if err != nil {
			t.Errorf(
				"List(%v, %s, %d, %d): returned error %s",
				testcase.tags, testcase.order, testcase.pageNum, testcase.pageSize,
				err.Error(),
			)
		}
		if want := testcase.want; !reflect.DeepEqual(want, have) {
			t.Errorf(
				"List(%v, %s, %d, %d): want %v, have %v",
				testcase.tags, testcase.order, testcase.pageNum, testcase.pageSize,
				want, have,
			)
		}
	}
}

func TestCatalogueServiceCount(t *testing.T) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	var cols []string = []string{"count"}

	mock.ExpectPrepare("SELECT *").ExpectQuery().WillReturnRows(sqlmock.NewRows(cols).AddRow(5))
	mock.ExpectPrepare("SELECT *").ExpectQuery().WillReturnRows(sqlmock.NewRows(cols).AddRow(4))
	mock.ExpectPrepare("SELECT *").ExpectQuery().WillReturnRows(sqlmock.NewRows(cols).AddRow(1))

	s := NewCatalogueService(sqlxDB, logger)
	for _, testcase := range []struct {
		tags []string
		want int
	}{
		{[]string{}, 5},
		{[]string{"prime"}, 4},
		{[]string{"even", "prime"}, 1},
	} {
		have, err := s.Count(testcase.tags)
		if err != nil {
			t.Errorf(
				"Count(%v): returned error %s",
				testcase.tags, err.Error(),
				err.Error(),
			)
		}
		if want := testcase.want; want != have {
			t.Errorf("Count(%v): want %d, have %d", testcase.tags, want, have)
		}
	}
}

func TestCatalogueServiceGet(t *testing.T) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	var cols []string = []string{"id", "name", "description", "price", "count", "image_url_1", "image_url_2", "tag_name"}

	// (Error) Test Cases 1
	mock.ExpectQuery("SELECT *").WillReturnRows(sqlmock.NewRows(cols))

	// Test Case 2
	mock.ExpectQuery("SELECT *").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(s3.ID, s3.Name, s3.Description, s3.Price, s3.Count, s3.ImageURL[0], s3.ImageURL[1], strings.Join(s3.Tags, ",")))

	s := NewCatalogueService(sqlxDB, logger)
	{
		// Error case
		for _, id := range []string{
			"0",
		} {
			want := ErrNotFound
			if _, have := s.Get(id); want != have {
				t.Errorf("Get(%s): want %v, have %v", id, want, have)
			}
		}
	}
	{
		// Success case
		for id, want := range map[string]Sock{
			"3": s3,
		} {
			have, err := s.Get(id)
			if err != nil {
				t.Errorf("Get(%s): %v", id, err)
				continue
			}
			if !reflect.DeepEqual(want, have) {
				t.Errorf("Get(%s): want %s, have %s", id, want.ID, have.ID)
				continue
			}
		}
	}
}

func TestCatalogueServiceStock(t *testing.T) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	var cols []string = []string{"count"}
	query := regexp.QuoteMeta(stockQuery)

	// Test Case 1
	mock.ExpectQuery(query).WithArgs(s3.ID).WillReturnRows(sqlmock.NewRows(cols).AddRow(s3.Count))

	// (Error) Test Case 2
	mock.ExpectQuery(query).WithArgs("0").WillReturnRows(sqlmock.NewRows(cols))

	// (Error) Test Case 3
	mock.ExpectQuery(query).WithArgs(s1.ID).WillReturnError(errors.New("connection refused"))

	s := NewCatalogueService(sqlxDB, logger)
	for _, testcase := range []struct {
		id      string
		want    int
		wantErr error
	}{
		{s3.ID, s3.Count, nil},
		{"0", 0, ErrNotFound},
		{s1.ID, 0, ErrDBConnection},
	} {
		have, err := s.Stock(testcase.id)
		if err != testcase.wantErr {
			t.Errorf("Stock(%s): want error %v, have %v", testcase.id, testcase.wantErr, err)
		}
		if have != testcase.want {
			t.Errorf("Stock(%s): want %d, have %d", testcase.id, testcase.want, have)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Stock: unmet SQL expectations: %s", err)
	}
}

func newMockService(t *testing.T) (Service, sqlmock.Sqlmock, func()) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	return NewCatalogueService(sqlx.NewDb(db, "sqlmock"), logger), mock, func() { db.Close() }
}

func TestCatalogueServiceAdjustStock(t *testing.T) {
	selectQuery := regexp.QuoteMeta(adjustSelectQuery)
	updateQuery := regexp.QuoteMeta(adjustUpdateQuery)
	insertQuery := regexp.QuoteMeta(adjustInsertQuery)
	countRows := func(n ...int) sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"count"})
		for _, c := range n {
			rows.AddRow(c)
		}
		return rows
	}

	for _, testcase := range []struct {
		name    string
		id      string
		delta   int
		reason  string
		expect  func(sqlmock.Sqlmock)
		want    int
		wantErr error
	}{
		{
			name: "commits the update and the adjustment row", id: "X", delta: -3, reason: "sale",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectQuery).WithArgs("X").WillReturnRows(countRows(10))
				mock.ExpectExec(updateQuery).WithArgs(7, "X").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(insertQuery).WithArgs("X", -3, "sale").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			want: 7,
		},
		{
			name: "allows stock to reach exactly zero", id: "X", delta: -2, reason: "sale",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectQuery).WithArgs("X").WillReturnRows(countRows(2))
				mock.ExpectExec(updateQuery).WithArgs(0, "X").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(insertQuery).WithArgs("X", -2, "sale").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			want: 0,
		},
		{
			name: "rolls back without writing when stock would go below zero", id: "X", delta: -5, reason: "sale",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectQuery).WithArgs("X").WillReturnRows(countRows(2))
				mock.ExpectRollback()
			},
			wantErr: ErrInsufficientStock,
		},
		{
			name: "rolls back without writing for an unknown id", id: "Y", delta: 1, reason: "restock",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectQuery).WithArgs("Y").WillReturnRows(countRows())
				mock.ExpectRollback()
			},
			wantErr: ErrNotFound,
		},
		{
			name: "rolls back when the update fails", id: "X", delta: 1, reason: "return",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectQuery).WithArgs("X").WillReturnRows(countRows(1))
				mock.ExpectExec(updateQuery).WillReturnError(errors.New("connection reset"))
				mock.ExpectRollback()
			},
			wantErr: ErrDBConnection,
		},
		{
			name: "rejects an unknown reason without touching the database", id: "X", delta: 1, reason: "gift",
			expect:  func(sqlmock.Sqlmock) {},
			wantErr: ErrInvalidReason,
		},
		{
			name: "rejects a zero delta without touching the database", id: "X", delta: 0, reason: "correction",
			expect:  func(sqlmock.Sqlmock) {},
			wantErr: ErrInvalidDelta,
		},
	} {
		s, mock, done := newMockService(t)
		testcase.expect(mock)
		have, err := s.AdjustStock(testcase.id, testcase.delta, testcase.reason)
		if err != testcase.wantErr {
			t.Errorf("AdjustStock %s: want error %v, have %v", testcase.name, testcase.wantErr, err)
		}
		if have != testcase.want {
			t.Errorf("AdjustStock %s: want %d, have %d", testcase.name, testcase.want, have)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("AdjustStock %s: unmet SQL expectations: %s", testcase.name, err)
		}
		done()
	}
}

func TestCatalogueServiceStockHistory(t *testing.T) {
	existsQuery := regexp.QuoteMeta(sockExistsQuery)
	query := regexp.QuoteMeta(historyQuery)
	cols := []string{"delta", "reason", "created_at"}

	if !strings.Contains(historyQuery, "WHERE sock_id =? ORDER BY created_at DESC, id DESC LIMIT 50") {
		t.Errorf("history query must filter by sock and return the newest 50 rows first, have %q", historyQuery)
	}

	// Three adjustments, newest first.
	s, mock, done := newMockService(t)
	mock.ExpectQuery(existsQuery).WithArgs("X").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(query).WithArgs("X").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(-3, "sale", int64(1792400000)).
		AddRow(10, "restock", int64(1792300000)).
		AddRow(1, "return", int64(1792200000)))
	have, err := s.StockHistory("X")
	if err != nil {
		t.Fatalf("StockHistory(X): unexpected error %v", err)
	}
	want := []StockAdjustment{
		{Delta: -3, Reason: "sale", CreatedAt: time.Unix(1792400000, 0).UTC()},
		{Delta: 10, Reason: "restock", CreatedAt: time.Unix(1792300000, 0).UTC()},
		{Delta: 1, Reason: "return", CreatedAt: time.Unix(1792200000, 0).UTC()},
	}
	if !reflect.DeepEqual(want, have) {
		t.Errorf("StockHistory(X): want %v, have %v", want, have)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("StockHistory(X): unmet SQL expectations: %s", err)
	}
	done()

	// A known sock with no adjustments returns an empty, non-nil slice.
	s, mock, done = newMockService(t)
	mock.ExpectQuery(existsQuery).WithArgs("X").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(query).WithArgs("X").WillReturnRows(sqlmock.NewRows(cols))
	have, err = s.StockHistory("X")
	if err != nil || have == nil || len(have) != 0 {
		t.Errorf("StockHistory(X) with no adjustments: want [] and no error, have %#v, %v", have, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("StockHistory(X) with no adjustments: unmet SQL expectations: %s", err)
	}
	done()

	// An unknown sock is not found, and its history is not queried.
	s, mock, done = newMockService(t)
	mock.ExpectQuery(existsQuery).WithArgs("Y").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	if _, err = s.StockHistory("Y"); err != ErrNotFound {
		t.Errorf("StockHistory(Y): want error %v, have %v", ErrNotFound, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("StockHistory(Y): unmet SQL expectations: %s", err)
	}
	done()

	// A database failure is reported as such.
	s, mock, done = newMockService(t)
	mock.ExpectQuery(existsQuery).WithArgs("X").WillReturnError(errors.New("connection refused"))
	if _, err = s.StockHistory("X"); err != ErrDBConnection {
		t.Errorf("StockHistory(X) with database down: want error %v, have %v", ErrDBConnection, err)
	}
	done()
}

func TestCatalogueServiceLowStock(t *testing.T) {
	query := regexp.QuoteMeta(lowStockQuery)
	cols := []string{"id", "name", "description", "price", "count", "image_url_1", "image_url_2", "tag_name"}

	if !strings.Contains(lowStockQuery, "WHERE sock.count <= ?") || !strings.Contains(lowStockQuery, "ORDER BY sock.count ASC") {
		t.Errorf("low-stock query must select count <= threshold ordered by count ascending, have %q", lowStockQuery)
	}

	// The database applies the threshold and order; the rows are mapped like List.
	out := s2
	out.Count = 0
	at := s5
	s, mock, done := newMockService(t)
	mock.ExpectQuery(query).WithArgs(5).WillReturnRows(sqlmock.NewRows(cols).
		AddRow(out.ID, out.Name, out.Description, out.Price, out.Count, out.ImageURL_1, out.ImageURL_2, out.TagString).
		AddRow(at.ID, at.Name, at.Description, at.Price, at.Count, at.ImageURL_1, at.ImageURL_2, at.TagString))
	have, err := s.LowStock(5)
	if err != nil {
		t.Fatalf("LowStock(5): unexpected error %v", err)
	}
	if want := []Sock{out, at}; !reflect.DeepEqual(want, have) {
		t.Errorf("LowStock(5): want %v, have %v", want, have)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("LowStock(5): unmet SQL expectations: %s", err)
	}
	done()

	// Nothing at or below the threshold returns an empty, non-nil slice.
	s, mock, done = newMockService(t)
	mock.ExpectQuery(query).WithArgs(0).WillReturnRows(sqlmock.NewRows(cols))
	have, err = s.LowStock(0)
	if err != nil || have == nil || len(have) != 0 {
		t.Errorf("LowStock(0): want [] and no error, have %#v, %v", have, err)
	}
	done()

	// A negative threshold is rejected without a query.
	s, mock, done = newMockService(t)
	if _, err = s.LowStock(-1); err != ErrInvalidThreshold {
		t.Errorf("LowStock(-1): want error %v, have %v", ErrInvalidThreshold, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("LowStock(-1): unmet SQL expectations: %s", err)
	}
	done()

	s, mock, done = newMockService(t)
	mock.ExpectQuery(query).WillReturnError(errors.New("connection refused"))
	if _, err = s.LowStock(5); err != ErrDBConnection {
		t.Errorf("LowStock(5) with database down: want error %v, have %v", ErrDBConnection, err)
	}
	done()
}

func TestCatalogueServiceTags(t *testing.T) {
	logger = log.NewLogfmtLogger(os.Stderr)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")

	var cols []string = []string{"name"}

	mock.ExpectQuery("SELECT name FROM tag").WillReturnRows(sqlmock.NewRows(cols).
		AddRow(tags[0]).
		AddRow(tags[1]).
		AddRow(tags[2]))

	s := NewCatalogueService(sqlxDB, logger)

	have, err := s.Tags()
	if err != nil {
		t.Errorf("Tags(): %v", err)
	}
	if !reflect.DeepEqual(tags, have) {
		t.Errorf("Tags(): want %v, have %v", tags, have)
	}
}

func TestCut(t *testing.T) {
	for _, testcase := range []struct {
		pageNum  int
		pageSize int
		want     []Sock
	}{
		{0, 1, []Sock{}}, // pageNum 0 is invalid
		{1, 0, []Sock{}}, // pageSize 0 is invalid
		{1, 1, []Sock{s1}},
		{1, 2, []Sock{s1, s2}},
		{1, 5, []Sock{s1, s2, s3, s4, s5}},
		{1, 9, []Sock{s1, s2, s3, s4, s5}},
		{2, 0, []Sock{}},
		{2, 1, []Sock{s2}},
		{2, 2, []Sock{s3, s4}},
		{2, 3, []Sock{s4, s5}},
		{2, 4, []Sock{s5}},
		{2, 5, []Sock{}},
		{2, 6, []Sock{}},
		{3, 0, []Sock{}},
		{3, 1, []Sock{s3}},
		{3, 2, []Sock{s5}},
		{3, 3, []Sock{}},
		{4, 1, []Sock{s4}},
		{4, 2, []Sock{}},
	} {
		have := cut(socks, testcase.pageNum, testcase.pageSize)
		if want := testcase.want; !reflect.DeepEqual(want, have) {
			t.Errorf("cut(%d, %d): want %s, have %s", testcase.pageNum, testcase.pageSize, printIDs(want), printIDs(have))
		}
	}
}

// Make test output nicer: just print sock IDs.
type printIDs []Sock

func (s printIDs) String() string {
	ids := make([]string, len(s))
	for i, ss := range s {
		ids[i] = ss.ID
	}
	return "[" + strings.Join(ids, ", ") + "]"
}
