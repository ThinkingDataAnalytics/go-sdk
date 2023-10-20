package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// TDDebugConsumer The data is reported one by one, and when an error occurs, the log will be printed on the console.
type TDDebugConsumer struct {
	serverUrl string // serverUrl
	appId     string // appId
	writeData bool   // is archive to TE
	deviceId  string // be used to debug in TE
}

// NewDebugConsumer init TDDebugConsumer
func NewDebugConsumer(serverUrl string, appId string) (TDConsumer, error) {
	return NewDebugConsumerWithWriter(serverUrl, appId, true)
}

func NewDebugConsumerWithWriter(serverUrl string, appId string, writeData bool) (TDConsumer, error) {
	return NewDebugConsumerWithDeviceId(serverUrl, appId, writeData, "")
}

func NewDebugConsumerWithDeviceId(serverUrl string, appId string, writeData bool, deviceId string) (TDConsumer, error) {
	// enable console log
	SetLogLevel(TDLogLevelDebug)

	if len(serverUrl) <= 0 {
		msg := fmt.Sprint("ServerUrl not be empty")
		tdLogError(msg)
		return nil, errors.New(msg)
	}

	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}

	u.Path = "/data_debug"

	c := &TDDebugConsumer{serverUrl: u.String(), appId: appId, writeData: writeData, deviceId: deviceId}

	tdLogInfo("Mode: debug consumer, appId: %s, serverUrl: %s", c.appId, c.serverUrl)

	return c, nil
}

func (c *TDDebugConsumer) Add(d Data) error {
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return err
	}

	var jsonStr string
	// if properties has includes complex data, SDK need parse time with regular expression
	if d.IsComplex {
		jsonStr = parseTime(jsonBytes)
	} else {
		jsonStr = string(jsonBytes)
	}

	tdLogInfo("%v", jsonStr)

	return c.send(jsonStr)
}

func (c *TDDebugConsumer) Flush() error {
	tdLogInfo("flush data")
	return nil
}

func (c *TDDebugConsumer) Close() error {
	tdLogInfo("debug consumer close")
	return nil
}

func (c *TDDebugConsumer) IsStringent() bool {
	return true
}

func (c *TDDebugConsumer) send(data string) error {
	var dryRun = "0"
	if !c.writeData {
		dryRun = "1"
	}
	postData := url.Values{"data": {data}, "appid": {c.appId}, "source": {"server"}, "dryRun": {dryRun}}
	if len(c.deviceId) > 0 {
		postData.Add("deviceId", c.deviceId)
	}
	resp, err := http.PostForm(c.serverUrl, postData)
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
			msg := fmt.Sprintf("send to receiver failed with return content:  %s", string(body))
			tdLogError(msg)
			return errors.New(msg)
		} else {
			tdLogInfo("send success: %v", result)
		}
	} else {
		return errors.New(fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))
	}
	return nil
}
