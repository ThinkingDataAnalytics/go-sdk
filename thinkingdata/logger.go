package thinkingdata

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// 日志打印前缀
const sdkLogPrefix = "[ThinkingAnalytics] "

// 输出到文件
var loggerFile *log.Logger

// 输出到控制台
var loggerStderr *log.Logger

// 日志配置
var logConfig LoggerConfig

// 创建一把全局锁
var loggerLock = new(sync.Mutex)

type LogType int32

// logger level
const (
	LoggerTypeOff               LogType = 1 << 0                                // 关闭日志
	LoggerTypePrint             LogType = 1 << 1                                // 控制台打印
	LoggerTypeWriteFile         LogType = 1 << 2                                // 输出到文件
	LoggerTypePrintAndWriteFile         = LoggerTypePrint | LoggerTypeWriteFile // 控制台打印，并输出到文件
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

// SetLoggerConfig 设置日志级别
func SetLoggerConfig(config LoggerConfig) {
	loggerLock.Lock()
	defer loggerLock.Unlock()
	// 赋值给全局的 logConfig 变量
	logConfig = config

	logType := logConfig.Type

	if logType < LoggerTypeOff || logType > LoggerTypePrintAndWriteFile {
		fmt.Println(sdkLogPrefix + "日志类型设置错误")
		return
	}

	if logType&LoggerTypeOff == LoggerTypeOff {
		// 关闭日志
		loggerStderr = nil
		loggerFile = nil
		return
	}

	if logType&LoggerTypePrint == LoggerTypePrint {
		// 输出到控制台
		var writer = os.Stderr
		loggerStderr = log.New(writer, sdkLogPrefix, log.LstdFlags|log.Lmsgprefix)
	}

	if logType&LoggerTypeWriteFile == LoggerTypeWriteFile {
		// 输出到文件
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
