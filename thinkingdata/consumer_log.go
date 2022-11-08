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
	ChannelSize              = 1000 // channel 缓冲区
	ROTATE_DAILY  RotateMode = 0    // 按天切分
	ROTATE_HOURLY RotateMode = 1    // 按小时切分
)

type LogConsumer struct {
	directory      string      // 日志文件存放目录
	dateFormat     string      // 与日志切分有关的时间格式
	fileSize       int64       // 单个日志文件大小，单位 MByte
	fileNamePrefix string      // 日志文件前缀名
	currentFile    *os.File    // 当前日志文件
	ch             chan string // 数据传输信道
	wg             sync.WaitGroup
}

type LogConfig struct {
	Directory      string     // 日志文件存放目录
	RotateMode     RotateMode // 与日志切分有关的时间格式
	FileSize       int        // 单个日志文件大小，单位 MByte
	FileNamePrefix string     // 日志文件前缀名
}

// NewLogConsumer 创建 LogConsumer. 传入日志目录和切分模式
func NewLogConsumer(directory string, r RotateMode) (Consumer, error) {
	return NewLogConsumerWithFileSize(directory, r, 0)
}

// NewLogConsumerWithFileSize 创建 LogConsumer. 传入日志目录和切分模式和单个文件大小限制
// directory: 日志文件存放目录
// r: 文件切分模式(按日切分、按小时切分)
// size: 单个日志文件上限，单位 MB
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

	c := &LogConsumer{
		directory:      config.Directory,
		dateFormat:     df,
		fileSize:       int64(config.FileSize * 1024 * 1024),
		fileNamePrefix: config.FileNamePrefix,
		ch:             make(chan string, ChannelSize),
	}
	return c, c.init()
}

func (c *LogConsumer) Add(d Data) error {
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return err
	}

	var jsonStr string
	// 判断property 中是否有复杂数据类型，如果无复杂数据类型，不需要正则替换时间
	if d.IsComplex {
		jsonStr = parseTime(jsonBytes)
	} else {
		jsonStr = string(jsonBytes)
	}

	Logger("%v", jsonStr)

	c.ch <- jsonStr
	return nil
}

func (c *LogConsumer) Flush() error {
	return c.currentFile.Sync()
}

func (c *LogConsumer) Close() error {
	close(c.ch)
	c.wg.Wait()
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
	// 是否需要分页
	if c.fileSize > 0 {
		return fmt.Sprintf("%s/%slog.%s_%d", c.directory, fileNamePrefix, timeStr, i)
	} else {
		return fmt.Sprintf("%s/%slog.%s", c.directory, fileNamePrefix, timeStr)
	}
}

// 开启一个 Go 程从信道中读入数据，并写入文件
func (c *LogConsumer) init() error {
	// 判断目录是否存在
	_, err := os.Stat(c.directory)
	if err != nil && os.IsNotExist(err) {
		e := os.MkdirAll(c.directory, os.ModePerm)
		if e != nil {
			return e
		}
	}
	timeStr := time.Now().Format(c.dateFormat)
	fd, err := os.OpenFile(c.constructFileName(timeStr, 0), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("open failed: %s\n", err)
		return err
	}
	c.currentFile = fd

	c.wg.Add(1)

	go func() {
		defer func() {
			if c.currentFile != nil {
				c.currentFile.Sync()
				c.currentFile.Close()
			}
			c.wg.Done()
		}()
		i := 0
		for {
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}

				timeStr := time.Now().Format(c.dateFormat)

				// 判断是否要切分日志: 根据切分模式和当前日志文件大小来判断
				var newName string
				fName := c.constructFileName(timeStr, i)
				if c.currentFile.Name() != fName {
					newName = fName
				} else if c.fileSize > 0 {
					stat, _ := c.currentFile.Stat()
					if stat.Size() > c.fileSize {
						i++
						newName = c.constructFileName(timeStr, i)
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
						Logger("open failed: %s\n", err)
						return
					}
				}

				_, err = fmt.Fprintln(c.currentFile, rec)
				if err != nil {
					Logger("LoggerWriter(%q): %s\n", c.currentFile.Name(), err)
					return
				}
			}
		}
	}()

	return nil
}
