package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	directory      string  // directory of log file
	dateFormat     string  // name format of log file
	fileSize       int64   // max size of single log file (MByte)
	fileNamePrefix string  // prefix of log file
	currentFile    logFile // owned exclusively by the writer goroutine
	done           chan struct{}
	ch             chan logCommand
	writeErr       error // first writer error; read outside writer only after done closes
	fileIndex      int
	mutex          *sync.RWMutex
	sdkClose       bool
}

// A flush command is ordered behind all previously accepted events.
type logCommand struct {
	data    []byte
	flushed chan error
}

type logFile interface {
	Write([]byte) (int, error)
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Close() error
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
	const maxFileSizeMB = int64((1<<63 - 1) / (1024 * 1024))
	if config.FileSize < 0 || int64(config.FileSize) > maxFileSizeMB {
		return nil, fmt.Errorf("invalid FileSize %d: must be between 0 and %d MB", config.FileSize, maxFileSizeMB)
	}

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
		fileSize:       int64(config.FileSize) * 1024 * 1024,
		fileNamePrefix: config.FileNamePrefix,
		done:           make(chan struct{}),
		ch:             make(chan logCommand, chanSize),
		mutex:          new(sync.RWMutex),
		sdkClose:       false,
	}

	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

// Add returns after admission; Flush or Close reports asynchronous file errors.
func (c *TDLogConsumer) Add(d Data) error {
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return err
	}
	// Keep admission protected until the send completes. The writer never takes
	// this lock, so a full queue can drain while Close waits for admitted senders.
	c.mutex.RLock()
	if c.sdkClose {
		c.mutex.RUnlock()
		return errors.New("add event failed, SDK has been closed")
	}
	c.ch <- logCommand{data: jsonBytes}
	c.mutex.RUnlock()
	// Invoke user loggers on the caller, outside the admission lock. A logger
	// may call Flush or Close, which must remain independent of the writer.
	if GetLogLevel() >= TDLogLevelInfo {
		tdLogInfo("Enqueue event data: %s", parseTime(jsonBytes))
	}
	return nil
}

// Flush waits for earlier accepted events to be written and synced.
// The first file error is retained and returned by subsequent Flush/Close calls.
func (c *TDLogConsumer) Flush() error {
	flushed := make(chan error, 1)
	c.mutex.RLock()
	if c.sdkClose {
		c.mutex.RUnlock()
		<-c.done
		return c.writeErr
	}
	c.ch <- logCommand{flushed: flushed}
	c.mutex.RUnlock()
	return <-flushed
}

// Close stops admission and waits for all accepted events and final file sync.
// Repeated and concurrent calls wait for the same result.
func (c *TDLogConsumer) Close() error {
	c.mutex.Lock()
	if !c.sdkClose {
		c.sdkClose = true
		close(c.ch)
	}
	c.mutex.Unlock()
	<-c.done
	return c.writeErr
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
		return err
	}
	c.currentFile = fd
	go c.run()
	return nil
}

func (c *TDLogConsumer) rememberError(err error) {
	if err != nil && c.writeErr == nil {
		c.writeErr = err
	}
}

func (c *TDLogConsumer) run() {
	defer close(c.done)
	for command := range c.ch {
		if command.flushed != nil {
			if c.currentFile != nil {
				c.rememberError(c.currentFile.Sync())
			}
			command.flushed <- c.writeErr
			continue
		}
		jsonStr := parseTime(command.data)
		c.rememberError(c.writeToFile(jsonStr))
	}
	if c.currentFile != nil {
		c.rememberError(c.currentFile.Sync())
		c.rememberError(c.currentFile.Close())
		c.currentFile = nil
	}
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
	if c.fileSize > 0 {
		if err := c.restoreFileIndex(timeStr); err != nil {
			return nil, err
		}
	}
	return c.openWritableLogFile(timeStr)
}

// Resume the latest numbered file for this period and prefix on restart.
// Older partial files and gaps must not move the append position backwards.
func (c *TDLogConsumer) restoreFileIndex(timeStr string) error {
	entries, err := ioutil.ReadDir(c.directory)
	if err != nil {
		return fmt.Errorf("read log directory: %w", err)
	}
	prefix := strings.TrimSuffix(filepath.Base(c.constructFileName(timeStr, 0)), "0")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name(), prefix)
		index, err := strconv.Atoi(suffix)
		// Only accept names generated by constructFileName, not backups or suffixes.
		if err == nil && index >= 0 && strconv.Itoa(index) == suffix && index > c.fileIndex {
			c.fileIndex = index
		}
	}
	return nil
}

// Check every candidate, including files left by a previous process. Called
// during initialization or by the writer, which exclusively owns fileIndex.
func (c *TDLogConsumer) openWritableLogFile(timeStr string) (*os.File, error) {
	for {
		file, err := os.OpenFile(c.constructFileName(timeStr, c.fileIndex), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		if c.fileSize <= 0 {
			return file, nil
		}
		stat, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("stat log file: %w", err)
		}
		if stat.Size() <= c.fileSize {
			return file, nil
		}
		// No writes were made to this candidate; close it before trying the next.
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close skipped log file: %w", err)
		}
		if c.fileIndex == int(^uint(0)>>1) {
			return nil, errors.New("log file index overflow")
		}
		c.fileIndex++
	}
}

// Called only by the writer goroutine, including rotation and error handling.
func (c *TDLogConsumer) writeToFile(str string) error {
	timeStr := time.Now().Format(c.dateFormat)
	fileName := c.constructFileName(timeStr, c.fileIndex)
	if c.currentFile != nil {
		if c.currentFile.Name() == fileName && c.fileSize > 0 {
			stat, err := c.currentFile.Stat()
			if err != nil {
				return fmt.Errorf("stat log file: %w", err)
			}
			if stat.Size() > c.fileSize {
				if c.fileIndex == int(^uint(0)>>1) {
					return errors.New("log file index overflow")
				}
				c.fileIndex++
				fileName = c.constructFileName(timeStr, c.fileIndex)
			}
		}
		if c.currentFile.Name() != fileName {
			syncErr := c.currentFile.Sync()
			closeErr := c.currentFile.Close()
			c.currentFile = nil
			if syncErr != nil {
				return fmt.Errorf("sync rotated log file: %w", syncErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close rotated log file: %w", closeErr)
			}
		}
	}
	if c.currentFile == nil {
		file, err := c.openWritableLogFile(timeStr)
		if err != nil {
			return err
		}
		c.currentFile = file
	}
	if _, err := fmt.Fprintln(c.currentFile, str); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}
	return nil
}

// Deprecated: please use TDLogConsumer
type LogConsumer struct {
	TDLogConsumer
}

// Deprecated: please use TDLogConsumerConfig
type LogConfig struct {
	TDLogConsumerConfig
}
