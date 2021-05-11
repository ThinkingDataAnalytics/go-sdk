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
	"sync"
	"time"
)

type BatchConsumer struct {
	serverUrl string         // 接收端地址
	appId     string         // 项目 APP ID
	timeout   time.Duration  // 网络请求超时时间, 单位毫秒
	compress  bool           // 是否数据压缩
	ch        chan batchData // 数据传输信道
	wg        sync.WaitGroup
	isClosed  bool // 是否关闭
}

type BatchConfig struct {
	ServerUrl string // 接收端地址
	AppId     string // 项目 APP ID
	BatchSize int    // 批量上传数目
	Timeout   int    // 网络请求超时时间, 单位毫秒
	Compress  bool   // 是否数据压缩
	AutoFlush bool   // 自动上传
	Interval  int    // 自动上传间隔，单位秒
}

// 内部数据传输信道的数据结构, 内部 Go 程接收并处理
type batchDataType int
type batchData struct {
	t batchDataType // 数据类型
	d Data          // 数据，当 t 为 TypeData 时有效
}

const (
	DefaultTimeOut   = 30000 // 默认超时时长 30 秒
	DefaultBatchSize = 20    // 默认批量发送条数
	MaxBatchSize     = 200   // 最大批量发送条数
	DefaultInterval  = 30    // 默认自动上传间隔 30 秒

	TypeData  batchDataType = 0 // 数据类型
	TypeFlush batchDataType = 1 // 立即发送数据
)

// 创建 BatchConsumer
func NewBatchConsumer(serverUrl string, appId string) (Consumer, error) {
	config := BatchConfig{
		ServerUrl: serverUrl,
		AppId:     appId,
		Compress:  true,
	}
	return initBatchConsumer(config)
}

// 创建指定批量发送条数的 BatchConsumer
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

// 创建指定压缩形式的 BatchConsumer
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

	var timeout int
	if config.Timeout == 0 {
		timeout = DefaultTimeOut
	} else {
		timeout = config.Timeout
	}

	c := &BatchConsumer{
		serverUrl: u.String(),
		appId:     config.AppId,
		timeout:   time.Duration(timeout) * time.Millisecond,
		compress:  config.Compress,
		ch:        make(chan batchData, ChannelSize),
	}

	c.wg.Add(1)

	var interval int
	if config.Interval == 0 {
		interval = DefaultInterval
	} else {
		interval = config.Interval
	}
	if config.AutoFlush {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		go func() {
			for {
				<-ticker.C
				_ = c.Flush()
				if c.isClosed {
					ticker.Stop()
				}
			}

		}()
	}

	go func() {
		buffer := make([]Data, 0, batchSize)

		defer func() {
			c.wg.Done()
		}()

		for {
			flush := false
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}

				switch rec.t {
				case TypeFlush:
					flush = true
				case TypeData:
					buffer = append(buffer, rec.d)
					if len(buffer) >= batchSize {
						flush = true
					}
				}
			}

			// 上传数据到服务端, 不会重试
			if flush && len(buffer) > 0 {
				jdata, err := json.Marshal(buffer)
				if err == nil {
					err = c.send(string(jdata), len(buffer))
					if err != nil {
						fmt.Println(err)
					}
				}
				buffer = buffer[:0]
				flush = false
			}
		}
	}()
	return c, nil
}

func (c *BatchConsumer) Add(d Data) error {
	c.ch <- batchData{
		t: TypeData,
		d: d,
	}
	return nil
}

func (c *BatchConsumer) Flush() error {
	c.ch <- batchData{
		t: TypeFlush,
	}
	return nil
}

func (c *BatchConsumer) Close() error {
	c.Flush()
	close(c.ch)
	c.wg.Wait()
	c.isClosed = true
	return nil
}

func (c *BatchConsumer) send(data string, size int) error {
	var encodedData string
	var err error
	var compressType = "gzip"
	if c.compress {
		encodedData, err = encodeData(data)
	} else {
		encodedData = data
		compressType = "none"
	}
	if err != nil {
		return err
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
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		var result struct {
			Code int
		}

		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}

		if result.Code != 0 {
			return errors.New(fmt.Sprintf("send to receiver failed with return code: %d", result.Code))
		}
	} else {
		return errors.New(fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))
	}
	return nil
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
