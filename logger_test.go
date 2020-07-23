package middlelogger

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type mockLogger struct {
	mtx      sync.Mutex
	cutoff   time.Duration
	multiple bool
	reqs     []*LogData
	panics   []*LogData
	slow     []*LogData
}

func (l *mockLogger) LogRequest(ld LogData) {
	l.mtx.Lock()
	l.reqs = append(l.reqs, &ld)
	l.mtx.Unlock()
}

// LogPanic is part of the PanicLogger interface.
func (l *mockLogger) LogPanic(ld LogData, err interface{}) {
	l.mtx.Lock()
	l.panics = append(l.panics, &ld)
	l.mtx.Unlock()
}

func (l *mockLogger) Cutoff(*http.Request) time.Duration {
	return l.cutoff
}

func (l *mockLogger) MultipleLogs(*http.Request) bool {
	return l.multiple
}

func (l *mockLogger) LogSlowRequest(ld LogData, i int) {
	l.mtx.Lock()
	l.slow = append(l.slow, &ld)
	l.mtx.Unlock()
}

func emptyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
}

func mockHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data [16]byte
		w.WriteHeader(status)
		w.Write(data[:])
	})
}

func panicHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data [16]byte
		w.WriteHeader(status)
		w.Write(data[:])
		panic("boo!")
	})
}

func slowHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data [8]byte
		w.WriteHeader(status)
		w.Write(data[:])
		time.Sleep(time.Millisecond * 17)
		w.Write(data[:])
	})
}

// TestLogsRequest tests that the logger middleware correctly logs a given
// request.
func TestLogsRequest(t *testing.T) {
	logg := &mockLogger{}
	status := 999
	middle := LoggerMiddleware(mockHandler(status), logg)
	r := httptest.NewRequest("", "/", nil)
	w := httptest.NewRecorder()
	middle.ServeHTTP(w, r)

	if len(logg.reqs) != 1 {
		t.Fatalf("unexpected nb of logged requests. want=1 got=%d",
			len(logg.reqs))
	}
	if logg.reqs[0].Status != status {
		t.Fatalf("unexpected logged status. want=%d got=%d", status,
			logg.reqs[0].Status)
	}
	if logg.reqs[0].BytesWritten != 16 {
		t.Fatalf("unexpected logged bytes written. want=16 got=%d",
			logg.reqs[0].BytesWritten)
	}
}

// TestLogsPanic tests that the logger middleware correctly logs a given panic.
func TestLogsPanic(t *testing.T) {
	logg := &mockLogger{}
	status := 999
	middle := LoggerMiddleware(panicHandler(status), logg)
	r := httptest.NewRequest("", "/", nil)
	w := httptest.NewRecorder()
	middle.ServeHTTP(w, r)

	if len(logg.panics) != 1 {
		t.Fatalf("unexpected nb of logged panics. want=1 got=%d",
			len(logg.panics))
	}
	if logg.panics[0].Status != status {
		t.Fatalf("unexpected logged status. want=%d got=%d", status,
			logg.panics[0].Status)
	}
	if logg.panics[0].BytesWritten != 16 {
		t.Fatalf("unexpected logged bytes written. want=16 got=%d",
			logg.panics[0].BytesWritten)
	}

}

// TestLogsSlow tests that the logger middleware correctly logs slow requests.
func TestLogsSlow(t *testing.T) {
	logg := &mockLogger{cutoff: time.Millisecond * 5, multiple: true}
	status := 999
	middle := LoggerMiddleware(slowHandler(status), logg)
	r := httptest.NewRequest("", "/", nil)
	w := httptest.NewRecorder()
	middle.ServeHTTP(w, r)

	// Given we're using a cutoff of 5ms and the slowHandler sleeps for
	// 17ms we expect at least 3 log messages, but we might get 4 if the
	// handler goroutine is very slow.

	if len(logg.slow) < 3 || len(logg.slow) > 4 {
		t.Fatalf("unexpected nb of logged slows. want=3-4 got=%d",
			len(logg.slow))
	}

	if logg.slow[2].Status != status {
		t.Fatalf("unexpected logged status. want=%d got=%d", status,
			logg.slow[2].Status)
	}

	// The last slow message should have only some of the written bytes.
	if logg.slow[2].BytesWritten != 8 {
		t.Fatalf("unexpected logged bytes written. want=8 got=%d",
			logg.slow[2].BytesWritten)
	}
}

type nullLogger struct {
	nb int
}

func (l *nullLogger) LogRequest(ld LogData) {
	l.nb++
}

// BenchmarkLogsRequest benchmarks the logging of requests.
func BenchmarkLogsRequest(b *testing.B) {
	logg := &nullLogger{}
	middle := LoggerMiddleware(emptyHandler(), logg)
	r := httptest.NewRequest("", "/", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		middle.ServeHTTP(w, r)
	}
	if logg.nb != b.N {
		b.Fatalf("incorrect number of logs. want=%d got=%d", b.N, logg.nb)
	}
}
