package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
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
	wg := sync.WaitGroup{}

	// 开启日志
	logConfig := thinkingdata.LoggerConfig{
		Type: thinkingdata.LoggerTypePrintAndWriteFile,
		Path: "./test.log",
	}
	thinkingdata.SetLoggerConfig(logConfig)

	// 创建按小时切分的 log consumer, 日志文件存放在当前目录
	config := thinkingdata.LogConfig{
		RotateMode:     thinkingdata.ROTATE_HOURLY,
		FileNamePrefix: "test_prefix",
		Directory:      "./log",
		FileSize:       2,
	}

	customData := A{
		"ThinkingData",
		time.Now(),
		[]B{
			{Trigger: "Now We Support", Time: time.Now()},
			{Trigger: "User Custom Struct Data", Time: time.Now()},
		},
	}

	consumer, err := thinkingdata.NewLogConsumerWithConfig(config)
	if err != nil {
		// consumer 初始化失败
	}
	ta := thinkingdata.New(consumer)

	ta.SetSuperProperties(map[string]interface{}{
		"super_is_date":   time.Now(),
		"super_is_bool":   true,
		"super_is_string": "hello",
		"super_is_num":    15.6,
	})

	ta.SetDynamicSuperProperties(func() map[string]interface{} {
		result := make(map[string]interface{})
		result["dynamic_super_name"] = "Tom"
		result["dynamic_super_time"] = time.Now()
		return result
	})

	accountId := "AA"
	distinctId := "ABCDEF123456"
	properties := map[string]interface{}{
		// "#time" 属性是系统预置属性，传入 datetime 对象，表示事件发生的时间，如果不填入该属性，则默认使用系统当前时间
		//"#time":time.Now(),
		"update_time": time.Now(),
		// "#ip" 属性是系统预置属性，如果服务端中能获取用户 IP 地址，并填入该属性，数数会自动根据 IP 地址解析用户的省份、城市信息
		"#ip":       "123.123.123.123",
		"channel":   "ta",       // 字符串
		"age":       1,          // 数字
		"isSuccess": true,       // 布尔
		"birthday":  time.Now(), // 时间
		"object": map[string]interface{}{
			"key": "value",
		}, // 对象
		"objectArr": []interface{}{
			map[string]interface{}{
				"key": "value",
			},
		}, // 对象组
		"arr":     []string{"测试1", "测试2", "测试3"}, // 数组
		"my_data": customData,                    // 自定义对象
		"time_1":  time.Now(),
		"time_2":  "2022-12-12T22:22:22.333444555Z",
		"time_3":  "2022-12-12T22:22:22.333+08:00",
		"time_4":  "2022-12-12T22:22:22.333Z",
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// track事件
			err := ta.Track(accountId, distinctId, "view_page", properties)
			if err != nil {
				fmt.Println(err)
			}

		}()
	}

	wg.Wait()

	ta.Flush()

	defer ta.Close()
}
