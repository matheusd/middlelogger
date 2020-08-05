package middlelogger

import (
	"net/http"
	"sync"
	"time"
)

// LogData is the data returned by the logger middleware so that clients can
// perform logging in an appropriate format and location.
type LogData struct {
	R            *http.Request
	W            http.ResponseWriter
	Status       int
	Start        time.Time
	TotalTime    time.Duration
	BytesWritten int64
}

// RequestLogger defines the interface that custom loggers need to offer.
type RequestLogger interface {
	LogRequest(LogData)
}

// PanicLogger defines the interface that clients that also wish to log panics
// need to offer.
type PanicLogger interface {
	LogPanic(LogData, interface{})
}

// SlowRequestLogger defines the interface that clients that also wish to log
// slow requests need to offer.
type SlowRequestLogger interface {
	Cutoff(*http.Request) time.Duration
	MultipleLogs(*http.Request) bool
	LogSlowRequest(LogData, int)
}

// loggedRequest maintains the state about a request that should be logged. It
// implements http.ResponseWriter so that the status code written to the client
// and any bytes sent can be accounted for.
type loggedRequest struct {
	w http.ResponseWriter

	// mtx protects the following fields.
	mtx          sync.Mutex
	status       int
	wroteHeader  bool
	bytesWritten int64
}

func (lr *loggedRequest) WriteHeader(code int) {
	lr.mtx.Lock()
	if lr.wroteHeader {
		lr.mtx.Unlock()
		return
	}
	lr.wroteHeader = true
	lr.status = code
	lr.mtx.Unlock()

	lr.w.WriteHeader(code)
}

func (lr *loggedRequest) Header() http.Header {
	return lr.w.Header()
}

func (lr *loggedRequest) Write(data []byte) (int, error) {
	written, err := lr.w.Write(data)

	lr.mtx.Lock()
	lr.bytesWritten += int64(written)
	lr.mtx.Unlock()
	return written, err
}

func (lr *loggedRequest) currentData() (int, int64) {
	lr.mtx.Lock()
	defer lr.mtx.Unlock()
	return lr.status, lr.bytesWritten
}

type logHandler struct {
	logger      RequestLogger
	panicLogger PanicLogger
	slowLogger  SlowRequestLogger
	next        http.Handler
}

// slowLog logs slow requests. It MUST be called as a goroutine and expects
// slowLogger to be filled.
func (lh *logHandler) slowLog(cutoff time.Duration, multiple bool,
	doneChan chan struct{}, w http.ResponseWriter, r *http.Request,
	start time.Time, lr *loggedRequest) {

	for i := 0; multiple || i == 0; i++ {
		select {
		case <-doneChan:
			// Done, so no more logging needed.
			return

		case <-time.After(cutoff):
			status, bytesWritten := lr.currentData()
			ld := LogData{
				R:            r,
				W:            w,
				Start:        start,
				TotalTime:    time.Since(start),
				Status:       status,
				BytesWritten: bytesWritten,
			}
			lh.slowLogger.LogSlowRequest(ld, i)
		}
	}
}

func (lh *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lr := &loggedRequest{w: w}
	start := time.Now()

	// Log slow requests if commanded to, so that we don't miss out some
	// log messages.
	var doneChan chan struct{}
	if lh.slowLogger != nil {
		cutoff := lh.slowLogger.Cutoff(r)
		multiple := lh.slowLogger.MultipleLogs(r)
		if cutoff > 0 {
			doneChan = make(chan struct{})
			go lh.slowLog(cutoff, multiple, doneChan, w, r, start, lr)
		}
	}

	// Log request once complete.
	defer func() {
		// Cancel the slow logger if needed.
		if doneChan != nil {
			close(doneChan)
		}

		status, bytesWritten := lr.currentData()
		ld := LogData{
			R:            r,
			W:            w,
			Start:        start,
			TotalTime:    time.Since(start),
			Status:       status,
			BytesWritten: bytesWritten,
		}

		// We _only_ attempt to recover from panics if a
		// panicLogger was specified, otherwise we might forbid
		// someone higher up the stack from catching and handling the
		// panic.
		if lh.panicLogger != nil {
			if err := recover(); err != nil {
				lh.panicLogger.LogPanic(ld, err)
				return
			}
		}

		lh.logger.LogRequest(ld)
	}()

	lh.next.ServeHTTP(lr, r)
}

// LoggerMiddleware is a middleware that provides callers with the ability to
// log relevant request and response data.
//
// The logger value will be called with the data for every request _after_ the
// request is completed.
//
// If logger also implements PanicLogger, then any panics that occur during the
// call to the next handler are recovered from and logged appropriately.
func LoggerMiddleware(next http.Handler, logger RequestLogger) http.Handler {
	panicLogger, _ := logger.(PanicLogger)
	slowLogger, _ := logger.(SlowRequestLogger)

	return &logHandler{
		logger:      logger,
		panicLogger: panicLogger,
		slowLogger:  slowLogger,
		next:        next,
	}
}
