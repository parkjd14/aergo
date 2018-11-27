package shared

import (
	stdlog "log"

	"github.com/aergoio/aergo-lib/log"
	hclog "github.com/hashicorp/go-hclog"
)

type Logger struct {
	hclog.Logger
	Wrapped *log.Logger
}

func (wrap *Logger) Trace(msg string, args ...interface{}) {
	wrap.Wrapped.Debug().Msg(msg)
}

func (wrap *Logger) Debug(msg string, args ...interface{}) {
	wrap.Wrapped.Debug().Msg(msg)
}

func (wrap *Logger) Warn(msg string, args ...interface{}) {
	wrap.Wrapped.Error().Msg(msg)
}

func (wrap *Logger) Error(msg string, args ...interface{}) {
	wrap.Wrapped.Error().Msg(msg)
}

func (wrap *Logger) Info(msg string, args ...interface{}) {
	wrap.Wrapped.Info().Msg(msg)
}

func (wrap *Logger) IsDebug() bool {
	return true
}

func (wrap *Logger) IsError() bool {
	return true
}

func (wrap *Logger) IsInfo() bool {
	return true
}

func (wrap *Logger) IsTrace() bool {
	return true
}

func (wrap *Logger) IsWarn() bool {
	return true
}

func (wrap *Logger) With(args ...interface{}) hclog.Logger {
	return wrap
}

func (wrap *Logger) Named(name string) hclog.Logger {
	return wrap
}

func (wrap *Logger) ResetNamed(name string) hclog.Logger {
	return nil
}

func (wrap *Logger) SetLevel(level hclog.Level) {
}

func (wrap *Logger) StandardLogger(opts *hclog.StandardLoggerOptions) *stdlog.Logger {
	return nil
}
