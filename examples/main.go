package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
	"github.com/google/uuid"
	"sync"
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
	//enable console log
	thinkingdata.SetLoggerConfig(thinkingdata.LoggerConfig{
		Type: thinkingdata.LoggerTypePrint,
	})

	// e.g. init consumer, you can choose different consumer

	//consumer, err := generateDebugConsumer() // DebugConsumer
	//consumer, err := generateBatchConsumer() // BatchConsumer
	consumer, err := generateLogConsumer() // LogConsumer
	if err != nil {
		// consumer init error
	}
	te := thinkingdata.New(consumer)

	defer func() {
		err = te.Close()
		if err != nil {
			fmt.Println("TE close error")
		} else {
			fmt.Println("TE close success")
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
	//syncExample(&te, accountId, distinctId, properties)

	//// async example
	asyncExample(&te, accountId, distinctId, properties)

	//// async with http server.
	//mock_server.Start(func() {
	//	asyncExample(&te, accountId, distinctId, properties)
	//})
}

func syncExample(te *thinkingdata.TDAnalytics, accountId, distinctId string, properties map[string]interface{}) {
	//sync
	for j := 0; j < 200000; j++ {
		distinctId = randomString()
		err := te.Track(accountId, distinctId, "view_page", properties)
		if err != nil {
			fmt.Println(err)
		}
	}
	err := te.Flush()
	if err == nil {
		fmt.Println("TE flush success")
	} else {
		fmt.Printf("TE flush error: %v", err.Error())
	}
}

func asyncExample(te *thinkingdata.TDAnalytics, accountId, distinctId string, properties map[string]interface{}) {
	wg := sync.WaitGroup{}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		distinctId = randomString()

		go func(index int, distinctId string) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				err := te.Track(accountId, distinctId, fmt.Sprintf("ab___%v___%v", index, j), properties)
				if err != nil {
					fmt.Println(err)
				}
			}
		}(i, distinctId)
	}
	wg.Wait()
	err := te.Flush()
	if err == nil {
		fmt.Println("TE flush success")
	} else {
		fmt.Printf("TE flush error: %v", err.Error())
	}
}

func randomString() string {
	newUUID, _ := uuid.NewUUID()
	return newUUID.String()
}

func generateLogConsumer() (thinkingdata.Consumer, error) {
	// logConsumer config
	config := thinkingdata.LogConfig{
		FileNamePrefix: "test_prefix",
		Directory:      "H:/log",
		RotateMode:     thinkingdata.ROTATE_HOURLY,
		FileSize:       200,
	}
	return thinkingdata.NewLogConsumerWithConfig(config)
}

func generateBatchConsumer() (thinkingdata.Consumer, error) {
	config := thinkingdata.BatchConfig{
		ServerUrl: "your serverUrl",
		AppId:     "your appId",
		AutoFlush: true,
		BatchSize: 100,
		Interval:  20,
	}
	return thinkingdata.NewBatchConsumerWithConfig(config)
}

func generateDebugConsumer() (thinkingdata.Consumer, error) {
	return thinkingdata.NewDebugConsumerWithDeviceId("url", "appid", false, "deviceId")
}
