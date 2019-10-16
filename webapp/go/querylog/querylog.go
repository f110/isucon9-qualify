package querylog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

type Logger interface {
	Log(d time.Duration, v string)
}

var (
	logger          Logger
	minimumDuration time.Duration
)

func init() {
	sql.Register("querylog", Driver{})
}

func InitWithStandardLogger(l *log.Logger) {
	logger = &loggerWithStandard{Logger: l}
}

func InitWithZap(l *zap.Logger) {
	logger = &loggerWithZap{Logger: l}
}

func InitWithWriter(w io.Writer) {
	logger = &loggerWithWriter{Writer: w}
}

func InitWithLogrus(l *logrus.Logger) {
	logger = &loggerWithLogrus{Logger: l}
}

func InitWithGlog() {
	logger = &loggerWithGlog{}
}

func SetMinimumDuration(d time.Duration) {
	minimumDuration = d
}

type loggerWithStandard struct {
	*log.Logger
}

func (l *loggerWithStandard) Log(d time.Duration, v string) {
	l.Printf("[%v] %s", d, v)
}

type loggerWithZap struct {
	*zap.Logger
}

func (l *loggerWithZap) Log(d time.Duration, v string) {
	l.Info("QueryLog", zap.Duration("duration", d), zap.String("query", v))
}

type loggerWithWriter struct {
	io.Writer
}

func (l *loggerWithWriter) Log(d time.Duration, v string) {
	l.Write([]byte("QueryLog [" + d.String() + "] " + v + "\n"))
}

type loggerWithLogrus struct {
	*logrus.Logger
}

func (l *loggerWithLogrus) Log(d time.Duration, v string) {
	l.WithFields(logrus.Fields{
		"duration": d,
		"query":    v,
	}).Info("QueryLog")
}

type loggerWithGlog struct{}

func (l *loggerWithGlog) Log(d time.Duration, v string) {
	glog.Infof("QueryLog [%v] %s", d, v)
}

type Driver struct{}

func (d Driver) Open(name string) (driver.Conn, error) {
	defer loggingQueryTime(time.Now(), name)

	conn, err := mysql.MySQLDriver{}.Open(name)
	if err == nil {
		return &Conn{internal: conn}, err
	}

	return nil, err
}

type Conn struct {
	internal driver.Conn
}

func (conn *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return conn.internal.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (conn *Conn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := conn.internal.Prepare(query)
	if err == nil {
		return &Stmt{internal: stmt, query: query}, err
	}

	return nil, err
}

func (conn *Conn) Close() error {
	return conn.internal.Close()
}

func (conn *Conn) Begin() (driver.Tx, error) {
	return conn.internal.Begin()
}

type Stmt struct {
	query    string
	internal driver.Stmt
}

func (stmt *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	defer stmt.loggingQueryTime(time.Now())

	return stmt.internal.(driver.StmtQueryContext).QueryContext(ctx, args)
}

func (stmt *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	defer stmt.loggingQueryTime(time.Now())

	return stmt.internal.(driver.StmtExecContext).ExecContext(ctx, args)
}

func (stmt *Stmt) Close() error {
	return stmt.internal.Close()
}

func (stmt *Stmt) NumInput() int {
	return stmt.internal.NumInput()
}

func (stmt *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return stmt.internal.Exec(args)
}

func (stmt *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return stmt.internal.Query(args)
}

func (stmt *Stmt) loggingQueryTime(t1 time.Time) {
	loggingQueryTime(t1, stmt.query)
}

func loggingQueryTime(t1 time.Time, v string) {
	if logger != nil {
		d := time.Now().Sub(t1)
		if minimumDuration <= d {
			logger.Log(d, v)
		}
	}
}
