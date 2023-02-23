package thinkingdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// DebugConsumer The data is reported one by one, and when an error occurs, the log will be printed on the console.
type DebugConsumer struct {
	serverUrl string // serverUrl
	appId     string // appId
	writeData bool   // is archive to TE
	deviceId  string // be used to debug in TE
}

// NewDebugConsumer init DebugConsumer
func NewDebugConsumer(serverUrl string, appId string) (Consumer, error) {
	return NewDebugConsumerWithWriter(serverUrl, appId, true)
}

func NewDebugConsumerWithWriter(serverUrl string, appId string, writeData bool) (Consumer, error) {
	return NewDebugConsumerWithDeviceId(serverUrl, appId, writeData, "")
}

func NewDebugConsumerWithDeviceId(serverUrl string, appId string, writeData bool, deviceId string) (Consumer, error) {
	// enable console log
	logConfig := LoggerConfig{
		Type: LoggerTypePrint,
	}
	SetLoggerConfig(logConfig)

	if len(serverUrl) <= 0 {
		msg := fmt.Sprint("ServerUrl not be empty")
		Logger(msg)
		return nil, errors.New(msg)
	}

	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, err
	}

	u.Path = "/data_debug"

	c := &DebugConsumer{serverUrl: u.String(), appId: appId, writeData: writeData, deviceId: deviceId}
	return c, nil
}

func (c *DebugConsumer) Add(d Data) error {
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
			return errors.New(fmt.Sprintf("send to receiver failed with return content:  %s", string(body)))
		}

	} else {
		return errors.New(fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))
	}
	return nil
}
