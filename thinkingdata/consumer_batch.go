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
	Timeout   time.Duration  // 网络请求超时时间, 单位毫秒
	compress  bool           //是否数据压缩
	ch        chan batchData // 数据传输信道
	wg        sync.WaitGroup
}

// 内部数据传输信道的数据结构, 内部 Go 程接收并处理
type batchDataType int
type batchData struct {
	t batchDataType // 数据类型
	d Data          // 数据，当 t 为 TYPE_DATA 时有效
}

const (
	DEFAULT_TIME_OUT   = 30000 // 默认超时时长 30 秒
	DEFAULT_BATCH_SIZE = 20    // 默认批量发送条数
	MAX_BATCH_SIZE     = 200   // 最大批量发送条数
	BATCH_CHANNEL_SIZE = 1000  // 数据缓冲区大小, 超过此值时会阻塞
	DEFAULT_COMPRESS   = true  //默认压缩gzip

	TYPE_DATA  batchDataType = 0 // 数据类型
	TYPE_FLUSH batchDataType = 1 // 立即发送数据
)

// 创建 BatchConsumer
func NewBatchConsumer(serverUrl string, appId string) (Consumer, error) {
	return initBatchConsumer(serverUrl, appId, DEFAULT_BATCH_SIZE, DEFAULT_TIME_OUT, DEFAULT_COMPRESS)
}

// 创建指定批量发送条数的 BatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// batchSize 批量发送条数
func NewBatchConsumerWithBatchSize(serverUrl string, appId string, batchSize int) (Consumer, error) {
	return initBatchConsumer(serverUrl, appId, batchSize, DEFAULT_TIME_OUT, DEFAULT_COMPRESS)
}

// 创建指定压缩形式的 BatchConsumer
// serverUrl 接收端地址
// appId 项目的 APP ID
// compress 是否压缩数据
func NewBatchConsumerWithCompress(serverUrl string, appId string, compress bool) (Consumer, error) {
	return initBatchConsumer(serverUrl, appId, DEFAULT_BATCH_SIZE, DEFAULT_TIME_OUT, compress)
}

func initBatchConsumer(serverUrl string, appId string, batchSize int, timeout int, compress bool) (Consumer, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/sync_server"

	if batchSize > MAX_BATCH_SIZE {
		batchSize = MAX_BATCH_SIZE
	}
	c := &BatchConsumer{
		serverUrl: u.String(),
		appId:     appId,
		Timeout:   time.Duration(timeout) * time.Millisecond,
		compress:  compress,
		ch:        make(chan batchData, CHANNEL_SIZE),
	}

	c.wg.Add(1)

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
				case TYPE_FLUSH:
					flush = true
				case TYPE_DATA:
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
		t: TYPE_DATA,
		d: d,
	}
	return nil
}

func (c *BatchConsumer) Flush() error {
	c.ch <- batchData{
		t: TYPE_FLUSH,
	}
	return nil
}

func (c *BatchConsumer) Close() error {
	c.Flush()
	close(c.ch)
	c.wg.Wait()
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
	req.Header.Set("version", SDK_VERSION)
	req.Header.Set("compress", compressType)
	req.Header["TA-Integration-Type"] = []string{LIB_NAME}
	req.Header["TA-Integration-Version"] = []string{SDK_VERSION}
	req.Header["TA-Integration-Count"] = []string{strconv.Itoa(size)}
	client := &http.Client{Timeout: c.Timeout}
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
