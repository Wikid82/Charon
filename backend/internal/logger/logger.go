package logger

import (
    "io"
    "os"

    "github.com/sirupsen/logrus"
)

var _log = logrus.New()

// Init initializes the global logger with output writer and debug level.
func Init(debug bool, out io.Writer) {
    if out == nil {
        out = os.Stdout
    }
    _log.SetOutput(out)
    if debug {
        _log.SetLevel(logrus.DebugLevel)
        _log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
    } else {
        _log.SetLevel(logrus.InfoLevel)
        _log.SetFormatter(&logrus.JSONFormatter{})
    }
}

// Log returns a standard logger entry to use across packages.
func Log() *logrus.Entry {
    return logrus.NewEntry(_log)
}

// WithFields returns a logger entry with provided fields.
func WithFields(fields logrus.Fields) *logrus.Entry {
    return Log().WithFields(fields)
}
