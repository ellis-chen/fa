package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Infof ...
func Infof(c context.Context, msg string, args ...interface{}) {
	FromContext(c).Infof(msg, args...)
}

// Debugf ...
func Debugf(c context.Context, msg string, args ...interface{}) {
	FromContext(c).Debugf(msg, args...)
}

// Warnf ...
func Warnf(c context.Context, msg string, args ...interface{}) {
	FromContext(c).Warnf(msg, args...)
}

// Errorf ...
func Errorf(c context.Context, msg string, args ...interface{}) {
	FromContext(c).Errorf(msg, args...)
}

type contextKey int

const (
	loggerKey contextKey = iota
)

// Logger the logger
type Logger interface {
	logrus.FieldLogger
	WriterLevel(logrus.Level) *io.PipeWriter
}

var (
	mainLogger  Logger
	logFilePath string
	logFile     *os.File
	logRotate   *lumberjack.Logger
)

func init() {
	mainLogger = logrus.StandardLogger()
	logrus.SetOutput(os.Stdout)
}

// SetLogger sets the logger.
func SetLogger(l Logger) {
	mainLogger = l
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	logrus.SetFormatter(formatter)
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return logrus.GetLevel()
}

// Str adds a string field.
func Str(key, value string) func(logrus.Fields) {
	return func(fields logrus.Fields) {
		fields[key] = value
	}
}

// With Adds fields.
func With(ctx context.Context, opts ...func(logrus.Fields)) context.Context {
	logger := FromContext(ctx)

	fields := make(logrus.Fields)
	for _, opt := range opts {
		opt(fields)
	}
	logger = logger.WithFields(fields)

	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext Gets the logger from context.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		panic("nil context")
	}

	logger, ok := ctx.Value(loggerKey).(Logger)
	if !ok {
		logger = mainLogger
	}

	return logger
}

// WithoutContext Gets the main logger.
func WithoutContext() Logger {
	return mainLogger
}

// OpenFile opens the log file using the specified path.
func OpenFile(path string) error {
	logFilePath = path

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	mw := io.MultiWriter(os.Stdout, logFile)
	logrus.SetOutput(mw)

	SetOutput(mw)

	return nil
}

// CloseFile closes the log and sets the Output to stdout.
func CloseFile() error {
	logrus.SetOutput(os.Stdout)

	if logFile != nil {
		return logFile.Close()
	}

	return nil
}

// RotateFile closes and reopens the log file to allow for rotation by an external source.
// If the log isn't backed by a file then it does nothing.
func RotateFile() error {
	logger := FromContext(context.Background())

	if logFile == nil && logFilePath == "" {
		logger.Debug("fa log is not writing to a file, ignoring rotate request")

		return nil
	}

	if logFile != nil {
		defer func(f *os.File) {
			_ = f.Close()
		}(logFile)
	}

	if err := OpenFile(logFilePath); err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}

	return nil
}

func SetRotateOut(l *lumberjack.Logger) {
	logRotate = l
	mw := io.MultiWriter(os.Stdout, l)
	logrus.SetOutput(mw)
}

func Rotate() error {
	if logRotate == nil {
		return fmt.Errorf("log rotate is not set")
	}
	logrus.Infof("Rotating log ...")

	return logRotate.Rotate()
}
