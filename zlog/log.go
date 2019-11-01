// Package log is a convenient way to call zap log funcs.

package zlog

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.SugaredLogger = NewLogger(NewConfigFromEnv())

type Config struct {
	// default log level is debug
	IsLogLevelInfo bool
	// default log to stderr
	LogFilePath string
	// whether to log simultaneously to both stderr and file
	IsNotLogBoth bool
	// whether to rotate log file at midnight
	IsNotLogRotate bool
}

func NewConfigFromEnv() Config {
	var c Config
	c.IsLogLevelInfo, _ = strconv.ParseBool(os.Getenv("LOG_LEVEL_INFO"))
	c.LogFilePath = os.Getenv("LOG_FILE_PATH")
	c.IsNotLogBoth, _ = strconv.ParseBool(os.Getenv("LOG_NOT_STDERR"))
	c.IsNotLogRotate, _ = strconv.ParseBool(os.Getenv("LOG_NOT_ROTATE"))
	return c
}

func NewLogger(conf Config) *zap.SugaredLogger {
	encoderConf := zap.NewProductionEncoderConfig()
	encoderConf.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConf)

	var writers []zapcore.WriteSyncer
	stdWriter, _, _ := zap.Open("stderr")
	if conf.LogFilePath == "" {
		writers = []zapcore.WriteSyncer{stdWriter}
	} else {
		var fileWriter zapcore.WriteSyncer
		if conf.IsNotLogRotate {
			fileWriter, _, _ = zap.Open(conf.LogFilePath)
		} else {
			fileWriter = zapcore.AddSync(NewTimedRotatingWriter(
				&lumberjack.Logger{Filename: conf.LogFilePath},
				/* interval */ 10*time.Millisecond,
			))
		}
		if conf.IsNotLogBoth {
			writers = []zapcore.WriteSyncer{fileWriter}
		} else {
			writers = []zapcore.WriteSyncer{stdWriter, fileWriter}
		}
	}
	combinedWriter := zap.CombineWriteSyncers(writers...)

	logLevel := zap.DebugLevel
	if conf.IsLogLevelInfo {
		logLevel = zap.InfoLevel
	}
	core := zapcore.NewCore(encoder, combinedWriter, logLevel)
	zl := zap.New(core, zap.AddCaller())
	zl = zl.WithOptions(zap.AddCallerSkip(1))
	logger := zl.Sugar()
	return logger
}

type TimedRotatingWriter struct {
	*lumberjack.Logger
	interval    time.Duration
	mutex       sync.RWMutex
	lastRotated time.Time
}

func NewTimedRotatingWriter(base *lumberjack.Logger, interval time.Duration) (
*TimedRotatingWriter) {
	w := &TimedRotatingWriter{Logger: base, interval: interval}
	w.mutex.Lock()
	w.lastRotated = time.Now().Truncate(interval)
	w.mutex.Unlock()
	return w
}

func (w *TimedRotatingWriter) rotateIfNeeded() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if time.Now().Sub(w.lastRotated) < w.interval {
		return nil
	}
	w.lastRotated = time.Now().Truncate(w.interval)
	fmt.Printf("%v about to rotate log file\n", w.lastRotated)
	err := w.Logger.Rotate()
	return err
}

func (w *TimedRotatingWriter) Write(p []byte) (int, error) {
	err := w.rotateIfNeeded()
	if err != nil {
		return 0, err
	}
	// ensure no goroutine write log while rotating
	w.mutex.RLock()
	n, err := w.Logger.Write(p)
	w.mutex.RUnlock()
	return n, err
}

func Fatal(args ...interface{}) {
	globalLogger.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	globalLogger.Fatalf(template, args...)
}

func Info(args ...interface{}) {
	globalLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	globalLogger.Infof(template, args...)
}

func Debug(args ...interface{}) {
	globalLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	globalLogger.Debugf(template, args...)
}

func Println(args ...interface{}) {
	globalLogger.Info(args...)
}

func Printf(template string, args ...interface{}) {
	globalLogger.Infof(template, args...)
}
