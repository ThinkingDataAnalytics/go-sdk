package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type RotateMode int32

const (
	DefaultChannelSize            = 1000 // channel size
	ROTATE_DAILY       RotateMode = 0    // by the day
	ROTATE_HOURLY      RotateMode = 1    // by the hour
)

// TDLogConsumer write data to file, it works with LogBus
type TDLogConsumer struct {
	directory      string   // directory of log file
	dateFormat     string   // name format of log file
	fileSize       int64    // max size of single log file (MByte)
	fileNamePrefix string   // prefix of log file
	currentFile    *os.File // current file handler
	wg             sync.WaitGroup
	ch             chan []byte
	mutex          *sync.RWMutex
	sdkClose       bool
}

type TDLogConsumerConfig struct {
	Directory      string     // directory of log file
	RotateMode     RotateMode // rotate mode of log file
	FileSize       int        // max size of single log file (MByte)
	FileNamePrefix string     // prefix of log file
	ChannelSize    int
}

func NewLogConsumer(directory string, r RotateMode) (TDConsumer, error) {
	return NewLogConsumerWithFileSize(directory, r, 0)
}

// NewLogConsumerWithFileSize init TDLogConsumer
// directory: directory of log file
// r: rotate mode of log file. (in days / hours)
// size: max size of single log file (MByte)
func NewLogConsumerWithFileSize(directory string, r RotateMode, size int) (TDConsumer, error) {
	config := TDLogConsumerConfig{
		Directory:  directory,
		RotateMode: r,
		FileSize:   size,
	}
	return NewLogConsumerWithConfig(config)
}

func NewLogConsumerWithConfig(config TDLogConsumerConfig) (TDConsumer, error) {
	var df string
	switch config.RotateMode {
	case ROTATE_DAILY:
		df = "2006-01-02"
	case ROTATE_HOURLY:
		df = "2006-01-02-15"
	default:
		errStr := "unknown rotate mode"
		tdLogInfo(errStr)
		return nil, errors.New(errStr)
	}

	chanSize := DefaultChannelSize
	if config.ChannelSize > 0 {
		chanSize = config.ChannelSize
	}

	c := &TDLogConsumer{
		directory:      config.Directory,
		dateFormat:     df,
		fileSize:       int64(config.FileSize * 1024 * 1024),
		fileNamePrefix: config.FileNamePrefix,
		wg:             sync.WaitGroup{},
		ch:             make(chan []byte, chanSize),
		mutex:          new(sync.RWMutex),
		sdkClose:       false,
	}

	return c, c.init()
}

func (c *TDLogConsumer) Add(d Data) error {
	var err error = nil
	c.mutex.Lock()
	if c.sdkClose {
		err = errors.New("add event failed, SDK has been closed")
	}
	c.mutex.Unlock()
	if err != nil {
		tdLogError(err.Error())
		return err
	}

	jsonBytes, jsonErr := json.Marshal(d)
	if jsonErr != nil {
		err = jsonErr
	} else {
		c.ch <- jsonBytes
	}
	return err
}

func (c *TDLogConsumer) Flush() error {
	tdLogInfo("flush data")
	var err error = nil
	c.mutex.Lock()
	if c.currentFile != nil {
		err = c.currentFile.Sync()
	}
	c.mutex.Unlock()
	return err
}

func (c *TDLogConsumer) Close() error {
	tdLogInfo("log consumer close")

	var err error = nil
	c.mutex.Lock()
	if c.sdkClose {
		err = errors.New("[ThinkingData][error]: SDK has been closed")
	} else {
		c.sdkClose = true
		close(c.ch)
	}
	c.mutex.Unlock()
	return err
}

func (c *TDLogConsumer) IsStringent() bool {
	return false
}

func (c *TDLogConsumer) constructFileName(timeStr string, i int) string {
	fileNamePrefix := ""
	if len(c.fileNamePrefix) != 0 {
		fileNamePrefix = c.fileNamePrefix + "."
	}
	// is need paging
	if c.fileSize > 0 {
		return fmt.Sprintf("%s/%slog.%s_%d", c.directory, fileNamePrefix, timeStr, i)
	} else {
		return fmt.Sprintf("%s/%slog.%s", c.directory, fileNamePrefix, timeStr)
	}
}

func (c *TDLogConsumer) init() error {
	fd, err := c.initLogFile()
	if err != nil {
		tdLogError("init log file failed: %s", err)
		return err
	}
	c.currentFile = fd

	go func() {
		defer func() {
			if c.currentFile != nil {
				_ = c.currentFile.Sync()
				err = c.currentFile.Close()
				c.currentFile = nil
			}
			tdLogInfo("Gracefully shutting down")
		}()
		for {
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}
				jsonStr := parseTime(rec)
				tdLogInfo("write event data: %s", jsonStr)
				c.writeToFile(jsonStr)
			}
		}
	}()

	tdLogInfo("Mode: log consumer, log path: " + c.directory)

	return nil
}

func (c *TDLogConsumer) initLogFile() (*os.File, error) {
	_, err := os.Stat(c.directory)
	if err != nil && os.IsNotExist(err) {
		e := os.MkdirAll(c.directory, os.ModePerm)
		if e != nil {
			return nil, e
		}
	}
	timeStr := time.Now().Format(c.dateFormat)
	return os.OpenFile(c.constructFileName(timeStr, 0), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
}

var logFileIndex = 0

func (c *TDLogConsumer) writeToFile(str string) {
	timeStr := time.Now().Format(c.dateFormat)
	// paging by Rotate Mode and current file size
	var newName string
	fName := c.constructFileName(timeStr, logFileIndex)

	if c.currentFile == nil {
		var openFileErr error
		c.mutex.Lock()
		c.currentFile, openFileErr = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		c.mutex.Unlock()
		if openFileErr != nil {
			tdLogInfo("open log file failed: %s\n", openFileErr)
			return
		}
	}

	if c.currentFile.Name() != fName {
		newName = fName
	} else if c.fileSize > 0 {
		stat, _ := c.currentFile.Stat()
		if stat.Size() > c.fileSize {
			logFileIndex++
			newName = c.constructFileName(timeStr, logFileIndex)
		}
	}
	if newName != "" {
		_ = c.currentFile.Sync()
		err := c.currentFile.Close()
		if err != nil {
			tdLogInfo("close file failed: %s\n", err)
			return
		}
		c.mutex.Lock()
		c.currentFile, err = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		c.mutex.Unlock()
		if err != nil {
			tdLogInfo("rotate log file failed: %s\n", err)
			return
		}
	}
	_, err := fmt.Fprintln(c.currentFile, str)
	if err != nil {
		tdLogInfo("LoggerWriter(%q): %s\n", c.currentFile.Name(), err)
		return
	}
}

// Deprecated: please use TDLogConsumer
type LogConsumer struct {
	TDLogConsumer
}

// Deprecated: please use TDLogConsumerConfig
type LogConfig struct {
	TDLogConsumerConfig
}
