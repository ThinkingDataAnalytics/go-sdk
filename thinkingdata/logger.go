package thinkingdata

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// log prefix
const sdkLogPrefix = "[ThinkingAnalytics] "

// to log file
var loggerFile *log.Logger

// to console
var loggerStderr *log.Logger

// logConfig
var logConfig LoggerConfig

// log mutex
var loggerLock = new(sync.Mutex)

type LogType int32

// logger level
const (
	LoggerTypeOff               LogType = 1 << 0                                // disable log
	LoggerTypePrint             LogType = 1 << 1                                // print on console
	LoggerTypeWriteFile         LogType = 1 << 2                                // print to file
	LoggerTypePrintAndWriteFile         = LoggerTypePrint | LoggerTypeWriteFile // print both on console and file
)

type LoggerConfig struct {
	Type LogType
	Path string
}

func Logger(format string, v ...interface{}) {
	if logConfig == (LoggerConfig{}) {
		return
	}

	logType := logConfig.Type

	if logType&LoggerTypePrint == LoggerTypePrint {
		if loggerStderr != nil {
			loggerStderr.Printf(format, v...)
		}
	}
	if logType&LoggerTypeWriteFile == LoggerTypeWriteFile {
		if loggerFile != nil {
			loggerFile.Printf(format, v...)
		}
	}
}

// SetLoggerConfig set config
func SetLoggerConfig(config LoggerConfig) {
	loggerLock.Lock()
	defer loggerLock.Unlock()
	// store logConfig
	logConfig = config

	logType := logConfig.Type

	if logType < LoggerTypeOff || logType > LoggerTypePrintAndWriteFile {
		fmt.Println(sdkLogPrefix + "log type error")
		return
	}

	if logType&LoggerTypeOff == LoggerTypeOff {
		// disable log
		loggerStderr = nil
		loggerFile = nil
		return
	}

	if logType&LoggerTypePrint == LoggerTypePrint {
		// print on console
		var writer = os.Stderr
		loggerStderr = log.New(writer, sdkLogPrefix, log.LstdFlags|log.Lmsgprefix)
	}

	if logType&LoggerTypeWriteFile == LoggerTypeWriteFile {
		// print to file
		if len(logConfig.Path) > 0 {
			fd, err := os.OpenFile(logConfig.Path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				fmt.Printf(sdkLogPrefix+"open failed: %s\n", err)
			} else {
				loggerFile = log.New(fd, sdkLogPrefix, log.LstdFlags|log.Lmsgprefix)
			}
		}
	}
}
