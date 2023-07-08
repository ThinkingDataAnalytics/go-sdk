package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type RotateMode int32

const (
	DefaultChannelSize            = 1000 // channel size
	ROTATE_DAILY       RotateMode = 0    // by the day
	ROTATE_HOURLY      RotateMode = 1    // by the hour
)

// LogConsumer write data to file, it works with LogBus
type LogConsumer struct {
	directory      string   // directory of log file
	dateFormat     string   // name format of log file
	fileSize       int64    // max size of single log file (MByte)
	fileNamePrefix string   // prefix of log file
	currentFile    *os.File // current file handler
	wg             sync.WaitGroup
	ch             chan []byte
	chStatus       int32 // ch 0:closed, 1:not closed
}

type LogConfig struct {
	Directory      string     // directory of log file
	RotateMode     RotateMode // rotate mode of log file
	FileSize       int        // max size of single log file (MByte)
	FileNamePrefix string     // prefix of log file
	ChannelSize    int
}

func NewLogConsumer(directory string, r RotateMode) (Consumer, error) {
	return NewLogConsumerWithFileSize(directory, r, 0)
}

// NewLogConsumerWithFileSize init LogConsumer
// directory: directory of log file
// r: rotate mode of log file. (in days / hours)
// size: max size of single log file (MByte)
func NewLogConsumerWithFileSize(directory string, r RotateMode, size int) (Consumer, error) {
	config := LogConfig{
		Directory:  directory,
		RotateMode: r,
		FileSize:   size,
	}
	return NewLogConsumerWithConfig(config)
}

func NewLogConsumerWithConfig(config LogConfig) (Consumer, error) {
	var df string
	switch config.RotateMode {
	case ROTATE_DAILY:
		df = "2006-01-02"
	case ROTATE_HOURLY:
		df = "2006-01-02-15"
	default:
		errStr := "unknown rotate mode"
		Logger(errStr)
		return nil, errors.New(errStr)
	}

	chanSize := DefaultChannelSize
	if config.ChannelSize > 0 {
		chanSize = config.ChannelSize
	}

	c := &LogConsumer{
		directory:      config.Directory,
		dateFormat:     df,
		fileSize:       int64(config.FileSize * 1024 * 1024),
		fileNamePrefix: config.FileNamePrefix,
		wg:             sync.WaitGroup{},
		ch:             make(chan []byte, chanSize),
		chStatus:       1,
	}
	return c, c.init()
}

func (c *LogConsumer) Add(d Data) error {
	if atomic.LoadInt32(&c.chStatus) == 0 {
		return errors.New("logConsumer is closed")
	}

	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return err
	}
	c.ch <- jsonBytes
	return nil
}

func (c *LogConsumer) Flush() error {
	return c.currentFile.Sync()
}

func (c *LogConsumer) Close() error {
	if !atomic.CompareAndSwapInt32(&c.chStatus, 1, 0) {
		return nil // if ch is closed, return
	}
	close(c.ch)
	c.wg.Wait()
	if c.currentFile != nil {
		c.currentFile.Sync()
		c.currentFile.Close()
		c.currentFile = nil
	}
	return nil
}

func (c *LogConsumer) IsStringent() bool {
	return false
}

func (c *LogConsumer) constructFileName(timeStr string, i int) string {
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

func (c *LogConsumer) init() error {
	fd, err := c.initLogFile()
	if err != nil {
		Logger("init log file failed: %s\n", err)
		return err
	}
	c.currentFile = fd

	c.wg.Add(1)

	go func() {
		defer func() {
			c.wg.Done()
		}()
		for {
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}
				jsonStr := parseTime(rec)
				c.writeToFile(jsonStr)
			}
		}
	}()

	return nil
}

func (c *LogConsumer) initLogFile() (*os.File, error) {
	_, err := os.Stat(c.directory)
	if err != nil && os.IsNotExist(err) {
		e := os.MkdirAll(c.directory, os.ModePerm)
		if e != nil {
			return nil, e
		}
	}
	timeStr := time.Now().Format(c.dateFormat)
	return os.OpenFile(c.constructFileName(timeStr, 0), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
}

var logFileIndex = 0

func (c *LogConsumer) writeToFile(str string) {
	timeStr := time.Now().Format(c.dateFormat)
	// paging by Rotate Mode and current file size
	var newName string
	fName := c.constructFileName(timeStr, logFileIndex)

	if c.currentFile == nil {
		var openFileErr error
		c.currentFile, openFileErr = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if openFileErr != nil {
			Logger("open log file failed: %s\n", openFileErr)
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
		err := c.currentFile.Close()
		if err != nil {
			Logger("close file failed: %s\n", err)
			return
		}
		c.currentFile, err = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			Logger("rotate log file failed: %s\n", err)
			return
		}
	}
	_, err := fmt.Fprintln(c.currentFile, str)
	if err != nil {
		Logger("LoggerWriter(%q): %s\n", c.currentFile.Name(), err)
		return
	}
}
