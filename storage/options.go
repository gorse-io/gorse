package storage

type Options struct {
	IsolationLevel string
}

type Option func(*Options)

func WithIsolationLevel(isolationLevel string) Option {
	return func(o *Options) {
		o.IsolationLevel = isolationLevel
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
