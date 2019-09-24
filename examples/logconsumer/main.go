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
		"SUPER_TIME":   time.Now(),
		"SUPER_BOOL":   true,
		"SUPER_STRING": "hello",
		"SUPER_NUM":    15.6,
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
				"#ip": "123.123.123.123",
				// 商品 ID
				"ProductId": "123456",
				// 商品类别
				"ProductCatalog": "Laptop Computer",
				// 是否加入收藏夹，Boolean 类型的属性
				"IsAddedToFav": true,
			}
			for i := 0; i < 10000; i++ {

				// 记录用户浏览商品事件
				err := ta.Track(account_id, distinct_id, "ViewProduct", properties)
				if err != nil {
					fmt.Println(err)
				}

				// 记录用户订单付款事件
				err = ta.Track(account_id, distinct_id, "PaidOrder", properties)
				if err != nil {
					fmt.Println(err)
				}
				ta.UserSet(account_id, distinct_id, map[string]interface{}{
					"USER_STRING": "haha",
					"USER_DATE":   time.Now(),
				})

			}
		}(fmt.Sprintf("thread%d", i))
	}

	//ta.UserDelete(account_id, distinct_id)

	wg.Wait()
	ta.Flush()

	defer ta.Close()
}
