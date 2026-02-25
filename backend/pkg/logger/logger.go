package logger

import (
	"os"

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
