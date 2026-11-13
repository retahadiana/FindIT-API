package helper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var ErrMailerTransient = errors.New("simulasi kegagalan sementara mail server")

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type LogMailer struct {
	path      string
	delay     time.Duration
	failEvery int64
	counter   atomic.Int64
	mu        sync.Mutex
}

func NewLogMailer(path string, delay time.Duration, failEvery int) *LogMailer {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return &LogMailer{path: path, delay: delay, failEvery: int64(failEvery)}
}

func (m *LogMailer) Send(ctx context.Context, to, subject, body string) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	n := m.counter.Add(1)
	if m.failEvery > 0 && n%m.failEvery == 0 {
		return ErrMailerTransient
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := os.OpenFile(m.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "[%s] TO=%s | SUBJECT=%s | BODY=%s\n", time.Now().Format(time.RFC3339), to, subject, body)
	return err
}
