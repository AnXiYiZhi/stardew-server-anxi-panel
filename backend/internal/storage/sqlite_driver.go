package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"

	sqlite "modernc.org/sqlite"
)

const sqliteInterruptCode = 9

var observedSQLiteDriverSequence atomic.Uint64

type interruptObserver struct {
	mu        sync.Mutex
	count     int
	limit     int
	triggered bool
	onLimit   func(int)
}

func newInterruptObserver(limit int, onLimit func(int)) *interruptObserver {
	return &interruptObserver{limit: limit, onLimit: onLimit}
}

func (o *interruptObserver) observe(err error) {
	if o == nil {
		return
	}

	o.mu.Lock()
	if isSQLiteInterrupt(err) {
		o.count++
	} else {
		o.count = 0
		o.triggered = false
	}
	count := o.count
	shouldTrigger := o.onLimit != nil && o.limit > 0 && count >= o.limit && !o.triggered
	if shouldTrigger {
		o.triggered = true
	}
	onLimit := o.onLimit
	o.mu.Unlock()

	if shouldTrigger {
		onLimit(count)
	}
}

func isSQLiteInterrupt(err error) bool {
	var sqliteErr interface{ Code() int }
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == sqliteInterruptCode
}

func registerObservedSQLiteDriver(observer *interruptObserver) string {
	name := "anxi-panel-sqlite-" + strconv.FormatUint(observedSQLiteDriverSequence.Add(1), 10)
	sql.Register(name, &observedSQLiteDriver{inner: &sqlite.Driver{}, observer: observer})
	return name
}

type observedSQLiteDriver struct {
	inner    driver.Driver
	observer *interruptObserver
}

func (d *observedSQLiteDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.inner.Open(name)
	if err != nil {
		d.observer.observe(err)
		return nil, err
	}
	return &observedSQLiteConn{Conn: conn, observer: d.observer}, nil
}

type observedSQLiteConn struct {
	driver.Conn
	observer *interruptObserver
}

func (c *observedSQLiteConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.Conn.Prepare(query)
	c.observer.observe(err)
	if err != nil {
		return nil, err
	}
	return &observedSQLiteStmt{Stmt: stmt, observer: c.observer}, nil
}

func (c *observedSQLiteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	conn, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	stmt, err := conn.PrepareContext(ctx, query)
	c.observer.observe(err)
	if err != nil {
		return nil, err
	}
	return &observedSQLiteStmt{Stmt: stmt, observer: c.observer}, nil
}

func (c *observedSQLiteConn) Begin() (driver.Tx, error) {
	tx, err := c.Conn.Begin()
	c.observer.observe(err)
	if err != nil {
		return nil, err
	}
	return &observedSQLiteTx{Tx: tx, observer: c.observer}, nil
}

func (c *observedSQLiteConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	conn, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		return c.Begin()
	}
	tx, err := conn.BeginTx(ctx, opts)
	c.observer.observe(err)
	if err != nil {
		return nil, err
	}
	return &observedSQLiteTx{Tx: tx, observer: c.observer}, nil
}

func (c *observedSQLiteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	conn, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := conn.ExecContext(ctx, query, args)
	c.observer.observe(err)
	return result, err
}

func (c *observedSQLiteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	conn, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := conn.QueryContext(ctx, query, args)
	if err != nil {
		c.observer.observe(err)
		return nil, err
	}
	return &observedSQLiteRows{Rows: rows, observer: c.observer}, nil
}

func (c *observedSQLiteConn) Ping(ctx context.Context) error {
	pinger, ok := c.Conn.(driver.Pinger)
	if !ok {
		return nil
	}
	err := pinger.Ping(ctx)
	c.observer.observe(err)
	return err
}

func (c *observedSQLiteConn) ResetSession(ctx context.Context) error {
	resetter, ok := c.Conn.(driver.SessionResetter)
	if !ok {
		return nil
	}
	err := resetter.ResetSession(ctx)
	c.observer.observe(err)
	return err
}

func (c *observedSQLiteConn) IsValid() bool {
	validator, ok := c.Conn.(driver.Validator)
	return !ok || validator.IsValid()
}

func (c *observedSQLiteConn) CheckNamedValue(value *driver.NamedValue) error {
	checker, ok := c.Conn.(driver.NamedValueChecker)
	if !ok {
		return driver.ErrSkip
	}
	return checker.CheckNamedValue(value)
}

type observedSQLiteStmt struct {
	driver.Stmt
	observer *interruptObserver
}

func (s *observedSQLiteStmt) Exec(args []driver.Value) (driver.Result, error) {
	result, err := s.Stmt.Exec(args)
	s.observer.observe(err)
	return result, err
}

func (s *observedSQLiteStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.Stmt.Query(args)
	if err != nil {
		s.observer.observe(err)
		return nil, err
	}
	return &observedSQLiteRows{Rows: rows, observer: s.observer}, nil
}

func (s *observedSQLiteStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	stmt, ok := s.Stmt.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := stmt.ExecContext(ctx, args)
	s.observer.observe(err)
	return result, err
}

func (s *observedSQLiteStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	stmt, ok := s.Stmt.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := stmt.QueryContext(ctx, args)
	if err != nil {
		s.observer.observe(err)
		return nil, err
	}
	return &observedSQLiteRows{Rows: rows, observer: s.observer}, nil
}

func (s *observedSQLiteStmt) ColumnConverter(index int) driver.ValueConverter {
	converter, ok := s.Stmt.(driver.ColumnConverter)
	if !ok {
		return driver.DefaultParameterConverter
	}
	return converter.ColumnConverter(index)
}

func (s *observedSQLiteStmt) CheckNamedValue(value *driver.NamedValue) error {
	checker, ok := s.Stmt.(driver.NamedValueChecker)
	if !ok {
		return driver.ErrSkip
	}
	return checker.CheckNamedValue(value)
}

type observedSQLiteRows struct {
	driver.Rows
	observer *interruptObserver
	once     sync.Once
}

func (r *observedSQLiteRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err != nil {
		r.once.Do(func() {
			if errors.Is(err, io.EOF) {
				r.observer.observe(nil)
				return
			}
			r.observer.observe(err)
		})
	}
	return err
}

func (r *observedSQLiteRows) Close() error {
	err := r.Rows.Close()
	r.once.Do(func() { r.observer.observe(err) })
	return err
}

func (r *observedSQLiteRows) HasNextResultSet() bool {
	rows, ok := r.Rows.(driver.RowsNextResultSet)
	return ok && rows.HasNextResultSet()
}

func (r *observedSQLiteRows) NextResultSet() error {
	rows, ok := r.Rows.(driver.RowsNextResultSet)
	if !ok {
		return io.EOF
	}
	err := rows.NextResultSet()
	if err != nil {
		r.once.Do(func() {
			if errors.Is(err, io.EOF) {
				r.observer.observe(nil)
				return
			}
			r.observer.observe(err)
		})
	}
	return err
}

func (r *observedSQLiteRows) ColumnTypeDatabaseTypeName(index int) string {
	rows, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName)
	if !ok {
		return ""
	}
	return rows.ColumnTypeDatabaseTypeName(index)
}

func (r *observedSQLiteRows) ColumnTypeLength(index int) (int64, bool) {
	rows, ok := r.Rows.(driver.RowsColumnTypeLength)
	if !ok {
		return 0, false
	}
	return rows.ColumnTypeLength(index)
}

func (r *observedSQLiteRows) ColumnTypeNullable(index int) (bool, bool) {
	rows, ok := r.Rows.(driver.RowsColumnTypeNullable)
	if !ok {
		return false, false
	}
	return rows.ColumnTypeNullable(index)
}

func (r *observedSQLiteRows) ColumnTypePrecisionScale(index int) (int64, int64, bool) {
	rows, ok := r.Rows.(driver.RowsColumnTypePrecisionScale)
	if !ok {
		return 0, 0, false
	}
	return rows.ColumnTypePrecisionScale(index)
}

func (r *observedSQLiteRows) ColumnTypeScanType(index int) reflect.Type {
	rows, ok := r.Rows.(driver.RowsColumnTypeScanType)
	if !ok {
		return nil
	}
	return rows.ColumnTypeScanType(index)
}

type observedSQLiteTx struct {
	driver.Tx
	observer *interruptObserver
}

func (t *observedSQLiteTx) Commit() error {
	err := t.Tx.Commit()
	t.observer.observe(err)
	return err
}

func (t *observedSQLiteTx) Rollback() error {
	err := t.Tx.Rollback()
	t.observer.observe(err)
	return err
}
