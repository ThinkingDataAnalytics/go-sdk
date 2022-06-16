// BatchConsumer 实现了批量同步的向接收端传送数据的功能
package thinkingdata

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type BatchConsumer struct {
	serverUrl   string        // 接收端地址
	appId       string        // 项目 APP ID
	timeout     time.Duration // 网络请求超时时间, 单位毫秒
	compress    bool          // 是否数据压缩
	bufferMutex *sync.RWMutex
	cacheMutex  *sync.RWMutex // 缓存锁

	buffer        []Data
	batchSize     int
	cacheBuffer   [][]Data // 缓存
	cacheCapacity int      // 缓存最大容量
}

type BatchConfig struct {
	ServerUrl     string // 接收端地址
	AppId         string // 项目 APP ID
	BatchSize     int    // 批量上传数目
	Timeout       int    // 网络请求超时时间, 单位毫秒
	Compress      bool   // 是否数据压缩
	AutoFlush     bool   // 自动上传
	Interval      int    // 自动上传间隔，单位秒
	CacheCapacity int    // 缓存最大容量
}

const (
	DefaultTimeOut       = 30000 // 默认超时时长 30 秒
	DefaultBatchSize     = 20    // 默认批量发送条数
	MaxBatchSize         = 200   // 最大批量发送条数
	DefaultInterval      = 30    // 默认自动上传间隔 30 秒
	DefaultCacheCapacity = 50
)

// NewBatchConsumer 创建 BatchConsumer
func NewBatchConsumer(serverUrl string, appId string) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
	}
	return initBatchConsumer(config)
}

// NewBatchConsumerWithBatchSize 创建指定批量发送条数的 BatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// batchSize 批量发送条数
func NewBatchConsumerWithBatchSize(serverUrl string, appId string, batchSize int) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
		BatchSize: batchSize,
	}
	return initBatchConsumer(config)
}

// NewBatchConsumerWithCompress 创建指定压缩形式的 BatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// compress 是否压缩数据
func NewBatchConsumerWithCompress(serverUrl string, appId string, compress bool) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  compress,
	}
	return initBatchConsumer(config)
}

func NewBatchConsumerWithConfig(config BatchConfig) (Consumer, error) {
	return initBatchConsumer(config)
}

func initBatchConsumer(config BatchConfig) (Consumer, error) {
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

	c := &BatchConsumer{
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
	}

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

func (c *BatchConsumer) Add(d Data) error {
	c.bufferMutex.Lock()
	c.buffer = append(c.buffer, d)
	c.bufferMutex.Unlock()

	if c.getBufferLength() >= c.batchSize || c.getCacheLength() > 0 {
		err := c.Flush()
		return err
	}

	return nil
}

func (c *BatchConsumer) Flush() error {
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

func (c *BatchConsumer) uploadEvents() error {
	buffer := c.cacheBuffer[0]

	jdata, err := json.Marshal(buffer)
	if err == nil {
		params := parseTime(jdata)
		for i := 0; i < 3; i++ {
			statusCode, code, err := c.send(params, len(buffer))
			if statusCode == 200 {
				c.cacheBuffer = c.cacheBuffer[1:]
				switch code {
				case 0:
					return nil
				case 1, -1:
					return fmt.Errorf("ThinkingDataError:invalid data format")
				case -2:
					return fmt.Errorf("ThinkingDataError:APP ID doesn't exist")
				case -3:
					return fmt.Errorf("ThinkingDataError:invalid ip transmission")
				default:
					return fmt.Errorf("ThinkingDataError:unknown error")
				}
			}
			if err != nil {
				if i == 2 {
					return err
				}
			}
		}
	}
	return nil
}

func (c *BatchConsumer) FlushAll() error {
	for c.getCacheLength() > 0 || c.getBufferLength() > 0 {
		if err := c.Flush(); err != nil {
			if !strings.Contains(err.Error(), "ThinkingDataError") {
				return err
			}
		}
	}
	return nil
}

func (c *BatchConsumer) Close() error {
	return c.FlushAll()
}

func (c *BatchConsumer) send(data string, size int) (statusCode int, code int, err error) {
	var encodedData string
	var compressType = "gzip"
	if c.compress {
		encodedData, err = encodeData(data)
	} else {
		encodedData = data
		compressType = "none"
	}
	if err != nil {
		return 0, 0, err
	}
	postData := bytes.NewBufferString(encodedData)

	var resp *http.Response
	req, _ := http.NewRequest("POST", c.serverUrl, postData)
	req.Header["appid"] = []string{c.appId}
	req.Header.Set("user-agent", "ta-go-sdk")
	req.Header.Set("version", SdkVersion)
	req.Header.Set("compress", compressType)
	req.Header["TA-Integration-Type"] = []string{LibName}
	req.Header["TA-Integration-Version"] = []string{SdkVersion}
	req.Header["TA-Integration-Count"] = []string{strconv.Itoa(size)}
	client := &http.Client{Timeout: c.timeout}
	resp, err = client.Do(req)

	if err != nil {
		return 0, 0, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		var result struct {
			Code int
		}

		err = json.Unmarshal(body, &result)
		if err != nil {
			return resp.StatusCode, 1, err
		}

		return resp.StatusCode, result.Code, nil
	} else {
		return resp.StatusCode, -1, nil
	}
}

// Gzip 压缩
func encodeData(data string) (string, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write([]byte(data))
	if err != nil {
		gw.Close()
		return "", err
	}
	gw.Close()

	return string(buf.Bytes()), nil
}

func (c *BatchConsumer) getBufferLength() int {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	return len(c.buffer)
}

func (c *BatchConsumer) getCacheLength() int {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	return len(c.cacheBuffer)
}
