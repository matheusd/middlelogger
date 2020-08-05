package middlelogger

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// StdLibLogger is a simple logger that logs all requests using the stdlib's
// log package. It implements Logger, PanicLogger and the SlowRequestLogger
// with a fixed cutoff time of 1 second.
//
// The logging format as ad-hoc.
type StdLibLogger struct{}

// LogRequest is part of the Logger interface.
func (l StdLibLogger) LogRequest(ld LogData) {
	log.Printf(
		"%s %s %d %s %d",
		ld.R.Method,
		ld.R.RequestURI,
		ld.Status,
		ld.TotalTime,
		ld.BytesWritten,
	)
}

// LogPanic is part of the PanicLogger interface.
func (l StdLibLogger) LogPanic(ld LogData, err interface{}) {
	log.Printf(
		"%s %s %d %s %d (PANIC %v)",
		ld.R.Method,
		ld.R.RequestURI,
		ld.Status,
		ld.TotalTime,
		ld.BytesWritten,
		err,
	)
	log.Printf(string(debug.Stack()))
}

func (l StdLibLogger) Cutoff(*http.Request) time.Duration {
	return time.Second
}

func (l StdLibLogger) MultipleLogs(*http.Request) bool {
	return true
}

func (l StdLibLogger) LogSlowRequest(ld LogData, i int) {
	log.Printf(
		"%s %s %d %s %d (slow %d)",
		ld.R.Method,
		ld.R.RequestURI,
		ld.Status,
		ld.TotalTime,
		ld.BytesWritten,
		i,
	)
}
