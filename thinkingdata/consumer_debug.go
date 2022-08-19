// DebugConsumer 逐条上传数据到接收端，并在出错时打印错误信息
package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type DebugConsumer struct {
	serverUrl string // 接收端地址
	appId     string // 项目 APP ID
	writeData bool   // 是否写入TA库
}

// 创建 DebugConsumer. DebugConsumer 实现逐条上报数据，并返回数据校验的详细错误信息.
func NewDebugConsumer(serverUrl string, appId string) (Consumer, error) {
	return NewDebugConsumerWithWriter(serverUrl, appId, true)
}
func NewDebugConsumerWithWriter(serverUrl string, appId string, writeData bool) (Consumer, error) {
	// 开启日志
	logConfig := LoggerConfig{
		Type: LoggerTypePrint,
	}
	SetLoggerConfig(logConfig)

	if len(serverUrl) <= 0 {
		msg := fmt.Sprint("ServerUrl 不能为空")
		Logger(msg)
		return nil, errors.New(msg)
	}

	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}

	u.Path = "/data_debug"

	c := &DebugConsumer{serverUrl: u.String(), appId: appId, writeData: writeData}
	return c, nil
}

func (c *DebugConsumer) Add(d Data) error {
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

	return c.send(jsonStr)
}

func (c *DebugConsumer) Flush() error {
	return nil
}

func (c *DebugConsumer) Close() error {
	return nil
}

func (c *DebugConsumer) IsStringent() bool {
	return true
}

func (c *DebugConsumer) send(data string) error {
	var dryRun = "0"
	if !c.writeData {
		dryRun = "1"
	}
	resp, err := http.PostForm(c.serverUrl, url.Values{"data": {data}, "appid": {c.appId}, "source": {"server"}, "dryRun": {dryRun}})
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		result := map[string]interface{}{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}
		if uint64(result["errorLevel"].(float64)) != 0 {
			return errors.New(fmt.Sprintf("send to receiver failed with return content:  %s", string(body)))
		}

	} else {
		return errors.New(fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))
	}
	return nil
}
