package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
	"sync"
	"time"
)

func main() {
	wg := sync.WaitGroup{}
	// 创建按小时切分的 log consumer, 日志文件存放在当前目录
	// consumer, _ := thinkingdata.NewLogConsumerWithFileSize(".", thinkingdata.ROTATE_HOURLY, 10)

	// 创建按天切分的 log consumer, 不设置单个日志上限
	consumer, _ := thinkingdata.NewLogConsumer(".", thinkingdata.ROTATE_DAILY)
	ta := thinkingdata.New(consumer)

	ta.SetSuperProperties(map[string]interface{}{
		"super_is_date":   time.Now(),
		"super_is_bool":   true,
		"super_is_string": "hello",
		"super_is_num":    15.6,
	})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(threadName string) {
			defer wg.Done()

			account_id := threadName
			distinct_id := "ABCDEF123456"
			properties := map[string]interface{}{
				// "#time" 属性是系统预置属性，传入 datetime 对象，表示事件发生的时间，如果不填入该属性，则默认使用系统当前时间
				//"#time":time.Now(),
				"update_time": time.Now(),
				// "#ip" 属性是系统预置属性，如果服务端中能获取用户 IP 地址，并填入该属性，数数会自动根据 IP 地址解析用户的省份、城市信息
				"#ip":     "123.123.123.123",
				"id":      "22",
				"catalog": "a",
				"is_boo":  true,
			}
			for i := 0; i < 20; i++ {

				// track事件
				err := ta.Track(account_id, distinct_id, "view_page", properties)
				if err != nil {
					fmt.Println(err)
				}
				ta.UserSet(account_id, distinct_id, map[string]interface{}{
					"is_string": "haha",
					"is_date":   time.Now(),
				})

			}
		}(fmt.Sprintf("thread%d", i))
	}

	//ta.UserDelete(account_id, distinct_id)

	wg.Wait()
	ta.Flush()

	defer ta.Close()
}
