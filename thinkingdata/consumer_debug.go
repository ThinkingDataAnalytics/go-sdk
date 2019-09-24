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
}

// 创建 DebugConsumer. DebugConsumer 实现逐条上报数据，并返回数据校验的详细错误信息.
func NewDebugConsumer(serverUrl string, appId string) (Consumer, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}

	u.Path = "/sync_data"

	c := &DebugConsumer{serverUrl: u.String(), appId: appId}
	return c, nil
}

func (c *DebugConsumer) Add(d Data) error {
	jdata, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return c.send(string(jdata))
}

func (c *DebugConsumer) Flush() error {
	return nil
}

func (c *DebugConsumer) Close() error {
	return nil
}

func (c *DebugConsumer) send(data string) error {
	resp, err := http.PostForm(c.serverUrl, url.Values{"data": {data}, "appid": {c.appId}, "debug": {"1"}})
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		var result struct {
			Code int
			Msg  string
		}

		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}

		if result.Code != 0 {
			return errors.New(fmt.Sprintf("send to receiver failed with return code: %d, due to %s", result.Code, result.Msg))
		}

	} else {
		return errors.New(fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))
	}
	return nil
}
