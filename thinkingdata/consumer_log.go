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
	CHANNEL_SIZE             = 1000 // channel 缓冲区
	ROTATE_DAILY  RotateMode = 0    // 按天切分
	ROTATE_HOURLY RotateMode = 1    // 按小时切分
)

type LogConsumer struct {
	directory   string      // 日志文件存放目录
	dateFormat  string      // 与日志切分有关的时间格式
	fileSize    int64       // 单个日志文件大小，单位 Byte
	currentFile *os.File    // 当前日志文件
	ch          chan string // 数据传输信道
	wg          sync.WaitGroup
}

// 创建 LogConsumer. 传入日志目录和切分模式
func NewLogConsumer(directory string, r RotateMode) (Consumer, error) {
	var df string
	switch r {
	case ROTATE_DAILY:
		df = "2006-01-02"
	case ROTATE_HOURLY:
		df = "2006-01-02-15"
	default:
		return nil, errors.New("Unknown rotate mode.")
	}

	c := &LogConsumer{
		directory:  directory,
		dateFormat: df,
		fileSize:   0, // 默认情况下不限制文件大小
		ch:         make(chan string, CHANNEL_SIZE),
	}
	return c, c.init()
}

// 创建 LogConsumer. 传入日志目录和切分模式和单个文件大小限制
// directory: 日志文件存放目录
// r: 文件切分模式(按日切分、按小时切分)
// size: 但个日志文件上限，单位 MB
func NewLogConsumerWithFileSize(directory string, r RotateMode, size int) (Consumer, error) {
	var df string
	switch r {
	case ROTATE_DAILY:
		df = "2006-01-02"
	case ROTATE_HOURLY:
		df = "2006-01-02-15"
	default:
		return nil, errors.New("Unknown rotate mode.")
	}

	if size < 0 {
		size = 0
	}

	c := &LogConsumer{
		directory:  directory,
		dateFormat: df,
		fileSize:   int64(size * 1024 * 1024),
		ch:         make(chan string, CHANNEL_SIZE),
	}
	return c, c.init()
}

func (c *LogConsumer) Add(d Data) error {
	bdata, err := json.Marshal(d)
	if err != nil {
		return err
	}

	c.ch <- string(bdata)
	return nil
}

func (c *LogConsumer) Flush() error {
	c.currentFile.Sync()
	return nil
}

func (c *LogConsumer) Close() error {
	close(c.ch)
	c.wg.Wait()
	return nil
}

func (c *LogConsumer) constructFileName(i int) string {
	if c.fileSize > 0 {
		return fmt.Sprintf("%s/log.%s_%d", c.directory, time.Now().Format(c.dateFormat), i)
	} else {
		return fmt.Sprintf("%s/log.%s", c.directory, time.Now().Format(c.dateFormat))
	}
}

// 开启一个 Go 程从信道中读入数据，并写入文件
func (c *LogConsumer) init() error {
	i := 0
	fd, err := os.OpenFile(c.constructFileName(i), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
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

		for {
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}

				// 判断是否要切分日志: 根据切分模式和当前日志文件大小来判断
				var newName string
				fname := c.constructFileName(i)
				if c.currentFile.Name() != fname {
					newName = fname
				} else if c.fileSize > 0 {
					stat, _ := c.currentFile.Stat()
					if stat.Size() > c.fileSize {
						i++
						newName = c.constructFileName(i)
					}
				}

				if newName != "" {
					c.currentFile.Close()
					c.currentFile, err = os.OpenFile(fname, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
					if err != nil {
						fmt.Printf("open failed: %s\n", err)
						return
					}
				}

				_, err = fmt.Fprintln(c.currentFile, rec)
				if err != nil {
					fmt.Fprintf(os.Stderr, "LoggerWriter(%q): %s\n", c.currentFile.Name(), err)
					return
				}
			}
		}
	}()

	return nil
}
