package backup

import (
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/kopia/kopia/repo/logging"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type kopiaLogger struct {
	src     logging.Logger
	rotator *lumberjack.Logger
}

func newKopiaLogger() *kopiaLogger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		// enc.AppendString(t.Format("2006-01-02T15:04:05.000Z0700"))
		enc.AppendString(t.Format("2006-01-02T15:04:05"))
	}

	logName := "/logs/kopia.log"
	conf := internal.LoadConf()
	if conf.Log.TermMode {
		logName = "logs/kopia.log"
	}

	rotatorLogger := &lumberjack.Logger{
		Filename:   logName,
		MaxSize:    10,
		MaxBackups: 50,
		MaxAge:     7,
		Compress:   false,
	}
	w := zapcore.AddSync(rotatorLogger)

	srcLogger := zap.New(
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
				// Keys can be anything except the empty string.
				TimeKey:        "T",
				LevelKey:       "L",
				NameKey:        "N",
				CallerKey:      zapcore.OmitKey,
				FunctionKey:    zapcore.OmitKey,
				MessageKey:     "M",
				StacktraceKey:  "S",
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.CapitalLevelEncoder,
				EncodeTime:     zapcore.ISO8601TimeEncoder,
				EncodeDuration: zapcore.StringDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}),
			// zapcore.NewJSONEncoder(cfg.EncoderConfig),
			zapcore.AddSync(w),
			zapcore.DebugLevel,
		),
	).Sugar()

	logrus.Infof("success setup kopia logger")

	return &kopiaLogger{
		src:     srcLogger,
		rotator: rotatorLogger,
	}
}

func (kl *kopiaLogger) close() (err error) {
	err = kl.src.Sync()
	if err != nil {
		return
	}

	return kl.rotator.Close()
}

//lint:ignore U1000 ignore such error
func (kl *kopiaLogger) rotate() (err error) {
	err = kl.src.Sync()
	if err != nil {
		return
	}

	return kl.rotator.Rotate()
}
