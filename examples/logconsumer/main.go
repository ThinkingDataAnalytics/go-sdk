package main

import (
	"fmt"
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
	// 创建按小时切分的 log consumer, 日志文件存放在当前目录
	// 创建按天切分的 log consumer, 不设置单个日志上限
	config := thinkingdata.LogConfig{
		FileNamePrefix: "test",
		Directory:      "/usr/log",
	}

	customData := A{
		"ThinkingData",
		time.Now(),
		[]B{
			{"Now We Support", time.Now()},
			{"User Custom Struct Data", time.Now()},
		},
	}

	consumer, _ := thinkingdata.NewLogConsumerWithConfig(config)
	ta := thinkingdata.New(consumer)

	ta.SetSuperProperties(map[string]interface{}{
		"super_is_date":   time.Now(),
		"super_is_bool":   true,
		"super_is_string": "hello",
		"super_is_num":    15.6,
	})

	accountId := "AA"
	distinctId := "ABCDEF123456"
	properties := map[string]interface{}{
		// "#time" 属性是系统预置属性，传入 datetime 对象，表示事件发生的时间，如果不填入该属性，则默认使用系统当前时间
		//"#time":time.Now(),
		"update_time": time.Now(),
		// "#ip" 属性是系统预置属性，如果服务端中能获取用户 IP 地址，并填入该属性，数数会自动根据 IP 地址解析用户的省份、城市信息
		"#ip":             "123.123.123.123",
		"id":              "22",
		"catalog":         "a",
		"is_boo":          true,
		"detect_results":  []string{"不通过"},
		"detect_type":     "image",
		"detect_way":      "YIDun",
		"label":           []string{"涉政"},
		"process_results": []string{"强制手机验证"},
		"source":          "group_chat",
		"my_data":         customData,
	}
	for i := 0; i < 100; i++ {

		// track事件
		err := ta.Track(accountId, distinctId, "view_page", properties)
		if err != nil {
			fmt.Println(err)
		}

	}
	ta.Flush()

	defer ta.Close()
}
