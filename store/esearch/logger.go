package esearch

import (
	"go.uber.org/zap"
)

// logger that changes printf to go to info for elastic library
type traceLogger struct {
	logger *zap.SugaredLogger
}

func (x *traceLogger) Printf(format string, vars ...interface{}) {
	x.logger.Debugf(format, vars...)
}

type infoLogger struct {
	logger *zap.SugaredLogger
}

func (x *infoLogger) Printf(format string, vars ...interface{}) {
	x.logger.Debugf(format, vars...) // The elastic info logger is noisy, send to debug
}

// logger that changes printf to go to error for elastic library
type errorLogger struct {
	logger *zap.SugaredLogger
}

func (x *errorLogger) Printf(format string, vars ...interface{}) {
	x.logger.Errorf(format, vars...)
}
