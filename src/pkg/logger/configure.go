package logger

import (
	"log"

	"github.com/onsi/ginkgo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	log *zap.Logger
}

func NewLogger(logLevel, app string) *Logger {
	cfg := zap.Config{
		Encoding:         "json",
		DisableCaller:    true,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:  "message",
			LevelKey:    "level",
			EncodeLevel: zapcore.LowercaseLevelEncoder,
			TimeKey:     "timestamp",
			EncodeTime:  zapcore.ISO8601TimeEncoder,
			NameKey:     "app",
		},
	}

	var logOption zapcore.Level
	switch logLevel {
	case "debug":
		logOption = zap.DebugLevel
		cfg.EncoderConfig.CallerKey = "caller"
		cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	case "info":
		logOption = zap.InfoLevel
	case "warn":
		logOption = zap.WarnLevel
	case "error":
		logOption = zap.ErrorLevel
	case "panic":
		logOption = zap.PanicLevel
	case "fatal":
		logOption = zap.FatalLevel
	default:
		logOption = zap.InfoLevel
	}

	cfg.Level = zap.NewAtomicLevelAt(logOption)

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	logger.Sync()

	namedLogger := logger.Named(app)
	return &Logger{
		log: namedLogger,
	}
}

func NewNop() *Logger {
	return &Logger{
		log: zap.NewNop(),
	}
}

func Count(count int) zap.Field {
	return zap.Int("count", count)
}

func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func (l *Logger) StdLog(name string) *log.Logger {
	return zap.NewStdLog(l.log.Named(name))
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.log.Info(msg, fields...)
}

func (l *Logger) Error(msg string, err error, extraFields ...zap.Field) {
	fields := []zap.Field{zap.Error(err)}
	fields = append(fields, extraFields...)
	l.log.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, err error) {
	l.log.Fatal(msg, zap.Error(err))
}

func (l *Logger) Panic(msg string, fields ...zap.Field) {
	l.log.Panic(msg, fields...)
}

func (l *Logger) Log(keyvals ...interface{}) error {
	var fields []zap.Field
	var msg string
	var key string

	nextIsValue := false
	for _, v := range keyvals {
		if nextIsValue == true {
			nextIsValue = false

			if key == "msg" {
				msg = v.(string)
				continue
			}

			fields = append(fields, zap.Any(key, v))
			continue
		}

		nextIsValue = true
		key = v.(string)
	}

	l.log.Info(msg, fields...)

	return nil
}

func NewTestLogger() *Logger {
	encoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey:   "message",
		LevelKey:     "level",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		TimeKey:      "timestamp",
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		CallerKey:    "caller",
		EncodeCaller: zapcore.ShortCallerEncoder,
	})
	logger := zap.New(zapcore.NewCore(
		encoder,
		zapcore.AddSync(ginkgo.GinkgoWriter),
		zapcore.DebugLevel,
	))
	logger.Sync()

	return &Logger{
		log: logger,
	}
}
