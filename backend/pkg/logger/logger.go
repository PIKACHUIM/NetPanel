package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var globalLogger *logrus.Logger

// Init 初始化全局日志
func Init() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	log.SetLevel(logrus.InfoLevel)
	globalLogger = log
	return log
}

// Get 获取全局日志实例
func Get() *logrus.Logger {
	if globalLogger == nil {
		return Init()
	}
	return globalLogger
}

// ─── 数据库日志写入 ────────────────────────────────────────────────────────────

// DBLogWriter 数据库日志写入接口，由 service/syslog 实现并注入
type DBLogWriter interface {
	Write(level, service, message string)
}

var (
	dbWriterMu sync.RWMutex
	dbWriter   DBLogWriter
)

// SetDBWriter 注入数据库日志写入器（在 main.go 初始化后调用）
func SetDBWriter(w DBLogWriter) {
	dbWriterMu.Lock()
	defer dbWriterMu.Unlock()
	dbWriter = w
}

// WriteLog 向数据库写入一条日志（同时输出到 stdout）
// level: info/warn/error/debug
// service: 服务标识，如 "system"/"frp"/"nps"/"easytier" 等
func WriteLog(level, service, message string) {
	log := Get()
	entry := log.WithField("service", service)
	switch level {
	case "debug":
		entry.Debug(message)
	case "warn", "warning":
		entry.Warn(message)
	case "error":
		entry.Error(message)
	default:
		entry.Info(message)
	}

	dbWriterMu.RLock()
	w := dbWriter
	dbWriterMu.RUnlock()
	if w != nil {
		w.Write(level, service, message)
	}
}

// ─── 带服务标签的 logrus.Logger 包装 ─────────────────────────────────────────

// ServiceLogger 带服务标签的日志器，写日志时同步写入数据库
type ServiceLogger struct {
	*logrus.Logger
	service string
}

// NewServiceLogger 创建带服务标签的日志器
func NewServiceLogger(base *logrus.Logger, service string) *ServiceLogger {
	return &ServiceLogger{Logger: base, service: service}
}

// DBHook logrus hook，将日志同步写入数据库
type DBHook struct {
	Service string // 服务标识，如 "system"/"frp"/"nps" 等
}

func (h *DBHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *DBHook) Fire(entry *logrus.Entry) error {
	dbWriterMu.RLock()
	w := dbWriter
	dbWriterMu.RUnlock()
	if w == nil {
		return nil
	}
	level := entry.Level.String()
	msg := entry.Message
	// 附加字段到消息
	if len(entry.Data) > 0 {
		for k, v := range entry.Data {
			if k == "service" {
				continue
			}
			msg += fmt.Sprintf(" %s=%v", k, v)
		}
	}
	go w.Write(level, h.Service, msg)
	return nil
}

// NewDBLogger 创建一个带数据库写入 hook 的 logrus.Logger
// 该 logger 的所有日志都会同步写入数据库，service 为服务标识
func NewDBLogger(base *logrus.Logger, service string) *logrus.Logger {
	l := logrus.New()
	l.SetLevel(base.Level)
	l.SetFormatter(base.Formatter)
	l.SetOutput(base.Out)
	l.AddHook(&DBHook{Service: service})
	return l
}

// ─── 时间工具 ─────────────────────────────────────────────────────────────────

// FormatTime 格式化时间
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}