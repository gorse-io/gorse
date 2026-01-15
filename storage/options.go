package storage

import (
	"database/sql"
	"time"
)

type Options struct {
	IsolationLevel  string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Option func(*Options)

func WithIsolationLevel(isolationLevel string) Option {
	return func(o *Options) {
		o.IsolationLevel = isolationLevel
	}
}

func WithMaxOpenConns(maxOpenConns int) Option {
	return func(o *Options) {
		o.MaxOpenConns = maxOpenConns
	}
}

func WithMaxIdleConns(maxIdleConns int) Option {
	return func(o *Options) {
		o.MaxIdleConns = maxIdleConns
	}
}

func WithConnMaxLifetime(connMaxLifetime time.Duration) Option {
	return func(o *Options) {
		o.ConnMaxLifetime = connMaxLifetime
	}
}

func ApplySQLPool(db *sql.DB, opt Options) {
	if opt.MaxOpenConns > 0 {
		db.SetMaxOpenConns(opt.MaxOpenConns)
	}
	if opt.MaxIdleConns > 0 {
		db.SetMaxIdleConns(opt.MaxIdleConns)
	}
	if opt.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(opt.ConnMaxLifetime)
	}
}

func NewOptions(opts ...Option) Options {
	opt := Options{
		IsolationLevel: "READ-UNCOMMITTED",
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}
