package sqlite

/*
#cgo darwin LDFLAGS: -lsqlite3
#cgo linux LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int spendable_bind_text(sqlite3_stmt *stmt, int index, const char *value) {
	return sqlite3_bind_text(stmt, index, value, -1, SQLITE_TRANSIENT);
}

static int spendable_bind_blob(sqlite3_stmt *stmt, int index, const void *value, int length) {
	return sqlite3_bind_blob(stmt, index, value, length, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"unsafe"
)

const sqliteDriverName = "spendable-sqlite3"

func init() {
	sql.Register(sqliteDriverName, &sqliteDriver{})
}

type sqliteDriver struct{}

func (*sqliteDriver) Open(name string) (driver.Conn, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var db *C.sqlite3
	flags := C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX
	if code := C.sqlite3_open_v2(cName, &db, C.int(flags), nil); code != C.SQLITE_OK {
		if db != nil {
			defer C.sqlite3_close(db)
		}
		return nil, sqliteError(db, code)
	}
	C.sqlite3_busy_timeout(db, 5000)
	return &sqliteConn{db: db}, nil
}

type sqliteConn struct {
	mu     sync.Mutex
	db     *C.sqlite3
	closed bool
}

func (c *sqliteConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not exposed")
}

func (c *sqliteConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if code := C.sqlite3_close(c.db); code != C.SQLITE_OK {
		return sqliteError(c.db, code)
	}
	return nil
}

func (c *sqliteConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *sqliteConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if opts.ReadOnly {
		return nil, errors.New("read-only transactions are not supported")
	}
	if _, err := c.ExecContext(context.Background(), "BEGIN IMMEDIATE", nil); err != nil {
		return nil, err
	}
	return &sqliteTx{conn: c}, nil
}

func (c *sqliteConn) Ping(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

func (c *sqliteConn) CheckNamedValue(value *driver.NamedValue) error {
	switch value.Value.(type) {
	case nil, int64, float64, bool, []byte, string:
		return nil
	case int:
		value.Value = int64(value.Value.(int))
		return nil
	default:
		return fmt.Errorf("unsupported sqlite parameter type %T", value.Value)
	}
}

func (c *sqliteConn) ExecContext(
	ctx context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, driver.ErrBadConn
	}
	if len(args) == 0 {
		cQuery := C.CString(query)
		defer C.free(unsafe.Pointer(cQuery))
		var message *C.char
		code := C.sqlite3_exec(c.db, cQuery, nil, nil, &message)
		if code != C.SQLITE_OK {
			if message != nil {
				defer C.sqlite3_free(unsafe.Pointer(message))
				return nil, fmt.Errorf("sqlite: %s", C.GoString(message))
			}
			return nil, sqliteError(c.db, code)
		}
		return sqliteResult{
			lastInsertID: int64(C.sqlite3_last_insert_rowid(c.db)),
			rowsAffected: int64(C.sqlite3_changes(c.db)),
		}, nil
	}

	stmt, err := prepare(c.db, query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindValues(stmt, args); err != nil {
		return nil, err
	}
	if code := C.sqlite3_step(stmt); code != C.SQLITE_DONE {
		return nil, sqliteError(c.db, code)
	}
	return sqliteResult{
		lastInsertID: int64(C.sqlite3_last_insert_rowid(c.db)),
		rowsAffected: int64(C.sqlite3_changes(c.db)),
	}, nil
}

func (c *sqliteConn) QueryContext(
	ctx context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, driver.ErrBadConn
	}
	stmt, err := prepare(c.db, query)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if err := bindValues(stmt, args); err != nil {
		C.sqlite3_finalize(stmt)
		c.mu.Unlock()
		return nil, err
	}

	count := int(C.sqlite3_column_count(stmt))
	columns := make([]string, count)
	for index := range count {
		columns[index] = C.GoString(C.sqlite3_column_name(stmt, C.int(index)))
	}
	return &sqliteRows{conn: c, stmt: stmt, columns: columns}, nil
}

type sqliteRows struct {
	conn    *sqliteConn
	stmt    *C.sqlite3_stmt
	columns []string
	closed  bool
}

func (r *sqliteRows) Columns() []string {
	return r.columns
}

func (r *sqliteRows) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	code := C.sqlite3_finalize(r.stmt)
	r.conn.mu.Unlock()
	if code != C.SQLITE_OK {
		return sqliteError(r.conn.db, code)
	}
	return nil
}

func (r *sqliteRows) Next(dest []driver.Value) error {
	if r.closed {
		return io.EOF
	}
	switch code := C.sqlite3_step(r.stmt); code {
	case C.SQLITE_ROW:
		for index := range len(dest) {
			column := C.int(index)
			switch C.sqlite3_column_type(r.stmt, column) {
			case C.SQLITE_NULL:
				dest[index] = nil
			case C.SQLITE_INTEGER:
				dest[index] = int64(C.sqlite3_column_int64(r.stmt, column))
			case C.SQLITE_FLOAT:
				dest[index] = float64(C.sqlite3_column_double(r.stmt, column))
			case C.SQLITE_BLOB:
				size := C.sqlite3_column_bytes(r.stmt, column)
				data := C.sqlite3_column_blob(r.stmt, column)
				dest[index] = C.GoBytes(data, size)
			default:
				size := C.sqlite3_column_bytes(r.stmt, column)
				text := C.sqlite3_column_text(r.stmt, column)
				dest[index] = C.GoStringN((*C.char)(unsafe.Pointer(text)), size)
			}
		}
		return nil
	case C.SQLITE_DONE:
		return io.EOF
	default:
		return sqliteError(r.conn.db, code)
	}
}

type sqliteTx struct {
	conn *sqliteConn
}

func (t *sqliteTx) Commit() error {
	_, err := t.conn.ExecContext(context.Background(), "COMMIT", nil)
	return err
}

func (t *sqliteTx) Rollback() error {
	_, err := t.conn.ExecContext(context.Background(), "ROLLBACK", nil)
	return err
}

type sqliteResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r sqliteResult) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

func (r sqliteResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func prepare(db *C.sqlite3, query string) (*C.sqlite3_stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var stmt *C.sqlite3_stmt
	if code := C.sqlite3_prepare_v2(db, cQuery, -1, &stmt, nil); code != C.SQLITE_OK {
		return nil, sqliteError(db, code)
	}
	return stmt, nil
}

func bindValues(stmt *C.sqlite3_stmt, args []driver.NamedValue) error {
	for _, arg := range args {
		index := C.int(arg.Ordinal)
		var code C.int
		switch value := arg.Value.(type) {
		case nil:
			code = C.sqlite3_bind_null(stmt, index)
		case int64:
			code = C.sqlite3_bind_int64(stmt, index, C.sqlite3_int64(value))
		case float64:
			code = C.sqlite3_bind_double(stmt, index, C.double(value))
		case bool:
			if value {
				code = C.sqlite3_bind_int64(stmt, index, 1)
			} else {
				code = C.sqlite3_bind_int64(stmt, index, 0)
			}
		case string:
			cValue := C.CString(value)
			code = C.spendable_bind_text(stmt, index, cValue)
			C.free(unsafe.Pointer(cValue))
		case []byte:
			if len(value) == 0 {
				code = C.spendable_bind_blob(stmt, index, nil, 0)
			} else {
				code = C.spendable_bind_blob(
					stmt,
					index,
					unsafe.Pointer(&value[0]),
					C.int(len(value)),
				)
			}
		default:
			return fmt.Errorf("unsupported sqlite parameter type %T", value)
		}
		if code != C.SQLITE_OK {
			return fmt.Errorf("sqlite bind parameter %d failed with code %d", arg.Ordinal, code)
		}
	}
	return nil
}

func sqliteError(db *C.sqlite3, code C.int) error {
	if db == nil {
		return fmt.Errorf("sqlite error code %d", code)
	}
	return fmt.Errorf("sqlite: %s (code %d)", C.GoString(C.sqlite3_errmsg(db)), code)
}
