package logging

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// L — общий логгер приложения.
var L = logrus.New()

type Options struct {
	Level  string
	Format string
	Output io.Writer
}

func Init(opt Options) {
	if opt.Output == nil {
		opt.Output = os.Stdout
	}
	L.SetOutput(opt.Output)

	lvl, err := logrus.ParseLevel(strings.ToLower(strings.TrimSpace(opt.Level)))
	if err != nil {
		lvl = logrus.InfoLevel
	}
	L.SetLevel(lvl)

	switch strings.ToLower(strings.TrimSpace(opt.Format)) {
	case "json":
		L.SetFormatter(&logrus.JSONFormatter{})
	default:
		L.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
}

// Совместимость со старым стилем вызова.
func Debugf(format string, args ...any) { L.Debugf(format, args...) }
func Infof(format string, args ...any)  { L.Infof(format, args...) }
func Warnf(format string, args ...any)  { L.Warnf(format, args...) }
func Errorf(format string, args ...any) { L.Errorf(format, args...) }
func Fatalf(format string, args ...any) { L.Fatalf(format, args...) }
