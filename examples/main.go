package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/v2/examples/mock_server"
	"github.com/ThinkingDataAnalytics/go-sdk/v2/src/thinkingdata"
	"github.com/google/uuid"
	"time"
)

type A struct {
	Name  string
	Time  time.Time
	Event []B
}

type B struct {
	Trigger string
	Time    time.Time
}

func main() {
	// enable console log
	thinkingdata.SetLogLevel(thinkingdata.TDLogLevelError)

	// e.g. init consumer, you can choose different consumer

	//consumer, err := generateDebugConsumer() // TDDebugConsumer
	//consumer, err := generateBatchConsumer() // TDBatchConsumer
	consumer, err := generateLogConsumer() // TDLogConsumer

	if err != nil {
		// consumer init error
	}
	te := thinkingdata.New(consumer)

	defer func() {
		err = te.Close()
		if err != nil {
			fmt.Printf("TE close error: %v\n", err.Error())
		} else {
			fmt.Println("[Example] TE close success")
		}
	}()

	te.SetSuperProperties(map[string]interface{}{
		"super_is_date":   time.Now(),
		"super_is_bool":   true,
		"super_is_string": "hello",
		"super_is_num":    15.6,
	})

	te.SetDynamicSuperProperties(func() map[string]interface{} {
		result := make(map[string]interface{})
		result["dynamic_super_name"] = "Tom"
		result["dynamic_super_time"] = time.Now()
		return result
	})

	customData := A{
		"ThinkingData",
		time.Now(),
		[]B{
			{Trigger: "Now We Support", Time: time.Now()},
			{Trigger: "User Custom Struct Data", Time: time.Now()},
		},
	}
	accountId := "AA"
	distinctId := "ABCDEF123456"
	properties := map[string]interface{}{
		// "#time" is preset properties in SDK. SDK would use default time if not set.
		"#time": time.Now(),
		// "#ip" is preset properties in SDK. SDK would use default ip address if not set.
		"#ip":       "123.123.123.123",
		"channel":   "ta",
		"age":       1,
		"isSuccess": true,
		"birthday":  time.Now(),
		"object": map[string]interface{}{
			"key": "value",
		},
		"objectArr": []interface{}{
			map[string]interface{}{
				"key": "value",
			},
		},

		"arr":     []string{"test1", "test2", "test3"},
		"my_data": customData,
		"time_1":  time.Now(),
		"time_2":  "2022-12-12T22:22:22.333444555Z",
		"time_3":  "2022-12-12T22:22:22.333+08:00",
		"time_4":  "2022-12-12T22:22:22.333Z",
	}

	// sync example
	syncExample(&te, accountId, distinctId, properties)

	// async with http server.
	mock_server.Start(&te)
}

func syncExample(te *thinkingdata.TDAnalytics, accountId, distinctId string, properties map[string]interface{}) {
	//sync
	for j := 0; j < 1; j++ {
		distinctId = randomString()
		err := te.Track(accountId, distinctId, "view_page", properties)
		if err != nil {
			fmt.Println(err)
		}
	}
	err := te.Flush()
	if err == nil {
		fmt.Println("[Example] TE flush success")
	} else {
		fmt.Printf("TE flush error: %v", err.Error())
	}
}

func randomString() string {
	newUUID, _ := uuid.NewUUID()
	return newUUID.String()
}

func generateLogConsumer() (thinkingdata.TDConsumer, error) {
	// logConsumer config
	config := thinkingdata.TDLogConsumerConfig{
		FileNamePrefix: "test_prefix",
		Directory:      "./log",
		RotateMode:     thinkingdata.ROTATE_HOURLY,
		FileSize:       1,
	}
	return thinkingdata.NewLogConsumerWithConfig(config)
}

func generateBatchConsumer() (thinkingdata.TDConsumer, error) {
	config := thinkingdata.TDBatchConfig{
		ServerUrl: "https://receiver-ta-uat.thinkingdata.cn",
		AppId:     "your appId",
		AutoFlush: true,
		BatchSize: 100,
		Interval:  20,
	}
	return thinkingdata.NewBatchConsumerWithConfig(config)
}

func generateDebugConsumer() (thinkingdata.TDConsumer, error) {
	return thinkingdata.NewDebugConsumerWithDeviceId("https://receiver-ta-uat.thinkingdata.cn", "appid", false, "deviceId")
}
