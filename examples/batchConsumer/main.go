package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
	"sync"
	"time"
)

func main() {
	wg := sync.WaitGroup{}
	// 创建 BatchConsumer, 指定接收端地址、APP ID、上报批次
	consumer, err := thinkingdata.NewBatchConsumer("url", "appid")
	// consumer, err := thinkingdata.NewBatchConsumerWithCompress("url", "appid",false)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 创建 TDAnalytics
	ta := thinkingdata.New(consumer)

	// 设置公共事件属性
	ta.SetSuperProperties(map[string]interface{}{
		"super_string": "value",
		"super_bool":   false,
	})

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
				"#ip":     "123.123.123.123",
				"id":      "12",
				//#uuid  去重，服务端比较稳定，可不填，如果填，按照以下标准8-4-4-4-12的String()
				"#uuid":    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
				"catalog": "p",
				"bool":    true,
			}
			for i := 0; i < 1; i++ {

				// track事件
				err := ta.Track(account_id, distinct_id, "ViewProduct", properties)
				if err != nil {
					fmt.Println(err)
				}

				ta.UserSet(account_id, distinct_id, map[string]interface{}{
					"user_string": "haha",
					"user_date":   time.Now(),
					"user_test":   "测试",
				})
			}
		}(fmt.Sprintf("thread%d", i))
	}

	wg.Wait()
	ta.Flush()
	defer ta.Close()
}
