package thinkingdata

import (
	"fmt"
	"sync"
	"time"
)

// SDK_LOG_PREFIX const
const SDK_LOG_PREFIX = "[ThinkingData]"

var logInstance TDLogger
var logMutex sync.RWMutex

type TDLogLevel int32

const (
	TDLogLevelOff     TDLogLevel = 1
	TDLogLevelError   TDLogLevel = 2
	TDLogLevelWarning TDLogLevel = 3
	TDLogLevelInfo    TDLogLevel = 4
	TDLogLevelDebug   TDLogLevel = 5
)

// default is TDLogLevelOff
var currentLogLevel = TDLogLevelOff

// TDLogger receives diagnostic messages. Print may be called concurrently.
// Panics from Print are contained so logging cannot interrupt event delivery.
type TDLogger interface {
	Print(message string)
}

// SetLogLevel Set the log output level
func SetLogLevel(level TDLogLevel) {
	if level < TDLogLevelOff || level > TDLogLevelDebug {
		fmt.Println(SDK_LOG_PREFIX + "log type error")
		return
	}
	logMutex.Lock()
	currentLogLevel = level
	logMutex.Unlock()
}

func GetLogLevel() TDLogLevel {
	logMutex.RLock()
	defer logMutex.RUnlock()
	return currentLogLevel
}

// SetCustomLogger Set a custom log input class, usually you don't need to set it up.
func SetCustomLogger(logger TDLogger) {
	if logger != nil {
		logMutex.Lock()
		logInstance = logger
		logMutex.Unlock()
	}
}

func tdLog(level TDLogLevel, format string, v ...interface{}) {
	logMutex.RLock()
	configuredLevel, logger := currentLogLevel, logInstance
	logMutex.RUnlock()
	if level > configuredLevel {
		return
	}

	var modeStr string
	switch level {
	case TDLogLevelError:
		modeStr = "[Error] "
		break
	case TDLogLevelWarning:
		modeStr = "[Warning] "
		break
	case TDLogLevelInfo:
		modeStr = "[Info] "
		break
	case TDLogLevelDebug:
		modeStr = "[Debug] "
		break
	default:
		modeStr = "[Info] "
		break
	}

	if logger != nil {
		msg := fmt.Sprintf(SDK_LOG_PREFIX+modeStr+format+"\n", v...)
		// Never hold configuration locks or log again while invoking user code.
		_ = withPanicRecovery(func() error {
			logger.Print(msg)
			return nil
		})
	} else {
		logTime := fmt.Sprintf("[%v]", time.Now().Format("2006-01-02 15:04:05.000"))
		fmt.Printf(logTime+SDK_LOG_PREFIX+modeStr+format+"\n", v...)
	}
}

func tdLogDebug(format string, v ...interface{}) {
	tdLog(TDLogLevelDebug, format, v...)
}

func tdLogInfo(format string, v ...interface{}) {
	tdLog(TDLogLevelInfo, format, v...)
}

func tdLogError(format string, v ...interface{}) {
	tdLog(TDLogLevelError, format, v...)
}

func tdLogWarning(format string, v ...interface{}) {
	tdLog(TDLogLevelWarning, format, v...)
}

// Deprecated: please use thinkingdata.SetLogLevel(thinkingdata.TDLogLevelOff)
type LogType int32

// Deprecated: please use thinkingdata.SetLogLevel(thinkingdata.TDLogLevelOff)
const (
	LoggerTypeOff               LogType = 1 << 0                                // disable log
	LoggerTypePrint             LogType = 1 << 1                                // print on console
	LoggerTypeWriteFile         LogType = 1 << 2                                // print to file
	LoggerTypePrintAndWriteFile         = LoggerTypePrint | LoggerTypeWriteFile // print both on console and file
)

// Deprecated: please use thinkingdata.SetLogLevel(thinkingdata.TDLogLevelOff)
type LoggerConfig struct {
	Type LogType
	Path string
}

// Deprecated: please use thinkingdata.SetLogLevel(thinkingdata.TDLogLevelOff)
func SetLoggerConfig(config LoggerConfig) {
	if config.Type < LoggerTypeOff || config.Type > LoggerTypePrintAndWriteFile {
		fmt.Println(SDK_LOG_PREFIX + "log type error")
		return
	}
	if config.Type&LoggerTypeOff == LoggerTypeOff {
		SetLogLevel(TDLogLevelOff)
	} else {
		SetLogLevel(TDLogLevelInfo)
	}
}
