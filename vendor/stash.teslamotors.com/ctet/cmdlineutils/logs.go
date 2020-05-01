package cmdlineutils

import (
	"flag"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var _levels = map[string]logrus.Level{
	logrus.TraceLevel.String(): logrus.TraceLevel,
	logrus.DebugLevel.String(): logrus.DebugLevel,
	logrus.InfoLevel.String():  logrus.InfoLevel,
	logrus.WarnLevel.String():  logrus.WarnLevel,
	logrus.ErrorLevel.String(): logrus.ErrorLevel,
	logrus.FatalLevel.String(): logrus.FatalLevel,
	logrus.PanicLevel.String(): logrus.PanicLevel,
}

// order is important here to construct the usage string
var _levelNames = []string{
	logrus.TraceLevel.String(),
	logrus.DebugLevel.String(),
	logrus.InfoLevel.String(),
	logrus.WarnLevel.String(),
	logrus.ErrorLevel.String(),
	logrus.FatalLevel.String(),
	logrus.PanicLevel.String(),
}

const _logLvlDef = logrus.InfoLevel

// ParseLogLevel takes a level string and converts it to a logrus.Level. If it
// cannot parse the lvl string it returns the default log level defined by _logLvlDef
func ParseLogLevel(lvl string) (logrus.Level, error) {
	return ParseLogLevelWithDefault(lvl, _logLvlDef)
}

// ParseLogLevelWithDefault takes a level string and converts it to a logrus.Level.
// If it cannot parse the lvl string it returns the default level defined by def
func ParseLogLevelWithDefault(lvl string, def logrus.Level) (logrus.Level, error) {
	var actual logrus.Level
	var err error

	actual, ok := _levels[strings.ToLower(lvl)]
	if !ok {
		actual = def
		err = fmt.Errorf("invalid level string %s", lvl)
	}

	return actual, err
}

// LogLevelUsageString returns the usage string to be used with the
// loglvl flag
func LogLevelUsageString() string {
	return strings.Join(_levelNames, "|")
}

const _logFlag = "loglvl"

// LogLevelFlagString returns the string to be used for a log level flag
func LogLevelFlagString() string {
	// this is a function in order to maintain the same API style as
	// LogLevelUsageString
	return _logFlag
}

// LogLevelFlag creates a default log level flag and returns the
// pointer to the user-inputted level string. Call this function before
// calling flag.Parse() in your cmdline program.
func LogLevelFlag() *string {
	return flag.String(LogLevelFlagString(), _logLvlDef.String(), LogLevelUsageString())
}

const (
	// LumberjackDefaultMaxMB is the maximum MB a log file will get before being rotated by lumberjack logger
	LumberjackDefaultMaxMB = 100
	// LumberjackDefaultMaxBackups is the maximum number of backups kept by lumberjack logger before being deleted
	LumberjackDefaultMaxBackups = 3
	// LumberjackDefaultMaxAge is the maximum age in days that lumberjack logger will keep old logs
	LumberjackDefaultMaxAge = 30
)

// SetupDefaultLumberjackLogrus returns a new configured *logrus.Logger with a default lumberjack logger
// as the output. The defaults used are the constants LumberjackDefaultMaxMB, LumberjackDefaultMaxBackups, and
// LumberjackDefaultMaxAge.
func SetupDefaultLumberjackLogrus(logFileName string, lvl logrus.Level) (*logrus.Logger, error) {
	return SetupLumberjackLogrus(logFileName, lvl, LumberjackDefaultMaxMB, LumberjackDefaultMaxBackups, LumberjackDefaultMaxAge)
}

// SetupLumberjackLogrus returns a new configured *logrus.Logger with a configured lumberjack logger as the output.
// Configure the lumberjack logger by supplying values to maxMB, maxBackups, and maxAge.
func SetupLumberjackLogrus(logFileName string, lvl logrus.Level, maxMB, maxBackups, maxAge int) (*logrus.Logger, error) {
	logger := logrus.New()
	logger.SetLevel(lvl)
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	ll := &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    maxMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true, // disabled by default
	}

	logger.SetOutput(ll)
	return logger, nil
}
