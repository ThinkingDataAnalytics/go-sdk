package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
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
	wg := sync.WaitGroup{}
	// 创建 AsyncBatchConsumer, 指定接收端地址、APP ID、上报批次
	config := thinkingdata.BatchConfig{
		ServerUrl: "http://ip:port",
		AppId:     "test",
		AutoFlush: false,
		BatchSize: 2,
		Interval:  3,
	}

	consumer, err := thinkingdata.NewAsyncBatchConsumerWithConfig(config)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 创建用户自定义信息类型
	customData := A{
		"ThinkingData",
		time.Now(),
		[]B{
			{"Now We Support", time.Now()},
			{"User Custom Struct Data", time.Now()},
		},
	}

	// 创建 TDAnalytics
	ta := thinkingdata.New(consumer)

	// 设置公共事件属性
	ta.SetSuperProperties(map[string]interface{}{
		"super_string": "value",
		"super_bool":   false,
	})

	fmt.Printf("%v", time.Now())
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(threadName string) {
			defer wg.Done()
			account_id := threadName
			distinct_id := "ABCDEF123456"
			properties := map[string]interface{}{
				// "#time" 属性是系统预置属性，传入 datetime 对象，表示事件发生的时间，如果不填入该属性，则默认使用系统当前时间
				"time_now": time.Now(),
				// "#ip" 属性是系统预置属性，如果服务端中能获取用户 IP 地址，并填入该属性，数数会自动根据 IP 地址解析用户的省份、城市信息
				"#ip": "123.123.123.123",
				"id":  "12",
				//#uuid  去重，服务端比较稳定，可不填，如果填，按照以下标准8-4-4-4-12的String()
				//"#uuid":   "6ba7b810-9dad-2233-80b4-00c04fd430c8",
				"catalog": "p",
				"bool":    true,
				"aa":      12,
				"my_data": customData,
			}
			// track事件
			err := ta.Track(account_id, distinct_id, "CancelOrder", properties)
			if err != nil {
				fmt.Println(err)
			}
		}(fmt.Sprintf("thread"))
	}

	fmt.Printf("%v \n", time.Now())

	time.Sleep(time.Second * 10)

	wg.Wait()
	ta.Flush()
	defer ta.Close()
}
