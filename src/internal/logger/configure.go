package logger

import (
	"io"
	"log"

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

func Int(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func (l *Logger) StdLog(name string) *log.Logger {
	return zap.NewStdLog(l.log.Named(name))
}

func (l *Logger) NamedLog(name string) *Logger {
	namedLogger := l.log.Named(name)
	return &Logger{
		log: namedLogger,
	}
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.log.Info(msg, fields...)
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.log.Debug(msg, fields...)
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

func (l *Logger) Sync() {
	l.log.Sync()
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

func NewTestLogger(w io.Writer) *Logger {
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	sync := zapcore.AddSync(w)

	core := zapcore.NewCore(
		encoder,
		sync,
		zap.LevelEnablerFunc(func(zapcore.Level) bool {
			return true
		}),
	)
	logger := zap.New(core)
	logger.Sync()

	return &Logger{
		log: logger,
	}
}
