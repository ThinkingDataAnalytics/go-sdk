package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AsyncBatchConsumer struct {
	BatchConsumer
}

// NewAsyncBatchConsumer 创建 AsyncBatchConsumer
func NewAsyncBatchConsumer(serverUrl string, appId string) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
	}
	return initAsyncBatchConsumer(config)
}

// NewAsyncBatchConsumerWithBatchSize 创建指定批量发送条数的 AsyncBatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// batchSize 批量发送条数
func NewAsyncBatchConsumerWithBatchSize(serverUrl string, appId string, batchSize int) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
		BatchSize: batchSize,
	}
	return initAsyncBatchConsumer(config)
}

// NewAsyncBatchConsumerWithCompress 创建指定压缩形式的 AsyncBatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// compress 是否压缩数据
func NewAsyncBatchConsumerWithCompress(serverUrl string, appId string, compress bool) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  compress,
	}
	return initAsyncBatchConsumer(config)
}

func NewAsyncBatchConsumerWithConfig(config BatchConfig) (Consumer, error) {
	return initAsyncBatchConsumer(config)
}

func initAsyncBatchConsumer(config BatchConfig) (Consumer, error) {
	if config.ServerUrl == "" {
		return nil, errors.New(fmt.Sprint("ServerUrl 不能为空"))
	}
	u, err := url.Parse(config.ServerUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/sync_server"

	var batchSize int
	if config.BatchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	} else if config.BatchSize <= 0 {
		batchSize = DefaultBatchSize
	} else {
		batchSize = config.BatchSize
	}

	var cacheCapacity int
	if config.CacheCapacity <= 0 {
		cacheCapacity = DefaultCacheCapacity
	} else {
		cacheCapacity = config.CacheCapacity
	}

	var timeout int
	if config.Timeout == 0 {
		timeout = DefaultTimeOut
	} else {
		timeout = config.Timeout
	}

	c := &AsyncBatchConsumer{BatchConsumer{
		serverUrl:     u.String(),
		appId:         config.AppId,
		timeout:       time.Duration(timeout) * time.Millisecond,
		compress:      config.Compress,
		bufferMutex:   new(sync.RWMutex),
		cacheMutex:    new(sync.RWMutex),
		batchSize:     batchSize,
		buffer:        make([]Data, 0, batchSize),
		cacheCapacity: cacheCapacity,
		cacheBuffer:   make([][]Data, 0, cacheCapacity),
	}}

	var interval int
	if config.Interval == 0 {
		interval = DefaultInterval
	} else {
		interval = config.Interval
	}
	if config.AutoFlush {
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Second)
			defer ticker.Stop()
			for {
				<-ticker.C
				_ = c.Flush()
			}
		}()
	}

	return c, nil
}

func (c *AsyncBatchConsumer) Add(d Data) error {
	c.bufferMutex.Lock()
	c.buffer = append(c.buffer, d)
	c.bufferMutex.Unlock()

	if c.getBufferLength() >= c.batchSize || c.getCacheLength() > 0 {
		err := c.Flush()
		return err
	}

	return nil
}

func (c *AsyncBatchConsumer) Flush() error {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	if len(c.buffer) == 0 && len(c.cacheBuffer) == 0 {
		return nil
	}

	defer func() {
		if len(c.cacheBuffer) > c.cacheCapacity {
			c.cacheBuffer = c.cacheBuffer[1:]
		}
	}()

	if len(c.cacheBuffer) == 0 || len(c.buffer) >= c.batchSize {
		c.cacheBuffer = append(c.cacheBuffer, c.buffer)
		c.buffer = make([]Data, 0, c.batchSize)
	}

	err := c.uploadEvents()

	return err
}

func (c *AsyncBatchConsumer) uploadEvents() error {
	// 取出将要上传的数据
	buffer := c.cacheBuffer[0]
	// 删除将要上传的数据
	c.cacheBuffer = c.cacheBuffer[1:]

	jdata, err := json.Marshal(buffer)
	if err == nil {
		params := parseTime(jdata)
		go func() {
			for i := 0; i < 3; i++ {
				statusCode, _, _ := c.send(params, len(buffer))
				if statusCode == 200 {
					break
				}
			}
		}()
	}
	return nil
}

func (c *AsyncBatchConsumer) FlushAll() error {
	for c.getCacheLength() > 0 || c.getBufferLength() > 0 {
		if err := c.Flush(); err != nil {
			if !strings.Contains(err.Error(), "ThinkingDataError") {
				return err
			}
		}
	}
	return nil
}

func (c *AsyncBatchConsumer) Close() error {
	return c.FlushAll()
}
