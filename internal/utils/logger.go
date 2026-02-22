// Package utils provides shared helper utilities.
package utils

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var logWriteMu sync.Mutex

// Logger はプレーンテキストの統一フォーマットでログを出力します。
type Logger struct {
	service string
}

// NewLogger は新しいロガーを作成します。
func NewLogger() *Logger {
	return &Logger{}
}

// WithService は service 属性付きロガーを返します。
func (l *Logger) WithService(service string) *Logger {
	return &Logger{service: service}
}

// Infof は INFO レベルのログを出力します。
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf("INFO", os.Stdout, format, args...)
}

// Warnf は WARN レベルのログを出力します。
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf("WARN", os.Stdout, format, args...)
}

// Errorf は ERROR レベルのログを出力します。
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf("ERROR", os.Stderr, format, args...)
}

func (l *Logger) logf(level string, out *os.File, format string, args ...interface{}) {
	message := strings.TrimRight(fmt.Sprintf(format, args...), "\n")

	prefix := fmt.Sprintf("%s [%s]", time.Now().Format(time.RFC3339), level)
	if l.service != "" {
		prefix += fmt.Sprintf(" [service=%s]", l.service)
	}

	logWriteMu.Lock()
	defer logWriteMu.Unlock()
	fmt.Fprintf(out, "%s %s\n", prefix, message)
}
