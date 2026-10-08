package thinkingdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TDDebugConsumer The data is reported one by one, and when an error occurs, the log will be printed on the console.
type TDDebugConsumer struct {
	serverUrl  string       // serverUrl
	appId      string       // appId
	writeData  bool         // is archive to TE
	deviceId   string       // be used to debug in TE
	HttpClient *http.Client // optional custom client; nonpositive timeout uses the SDK default
}

// NewDebugConsumer init TDDebugConsumer
func NewDebugConsumer(serverUrl string, appId string) (TDConsumer, error) {
	return NewDebugConsumerWithWriter(serverUrl, appId, true)
}

func NewDebugConsumerWithWriter(serverUrl string, appId string, writeData bool) (TDConsumer, error) {
	return NewDebugConsumerWithDeviceId(serverUrl, appId, writeData, "")
}

func NewDebugConsumerWithDeviceId(serverUrl string, appId string, writeData bool, deviceId string) (TDConsumer, error) {
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

	c := &TDDebugConsumer{serverUrl: u.String(), appId: appId, writeData: writeData, deviceId: deviceId, HttpClient: &http.Client{Timeout: time.Duration(DefaultTimeOut) * time.Millisecond}}

	// Enable debug logging only after initialization succeeds.
	SetLogLevel(TDLogLevelDebug)
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
	return nil
}

func (c *TDDebugConsumer) Close() error {
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
	client := c.HttpClient
	if client == nil {
		client = &http.Client{}
	}
	timeout := client.Timeout
	if timeout <= 0 {
		timeout = time.Duration(DefaultTimeOut) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverUrl, strings.NewReader(postData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read debug response: %w", err)
		}
		var result struct {
			ErrorLevel *int `json:"errorLevel"`
		}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}
		if result.ErrorLevel == nil {
			return errors.New("invalid debug response: errorLevel is missing or null")
		}
		if *result.ErrorLevel != 0 {
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
