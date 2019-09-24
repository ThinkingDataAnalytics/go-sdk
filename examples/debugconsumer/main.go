package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
	"time"
)

func main() {
	var err error

	// 创建 Debug Consumer
	consumer, _ := thinkingdata.NewDebugConsumer("https://sdk.tga.thinkinggame.cn", "b2a61feb9e56472c90c5bcb320dfb4ef")
	ta := thinkingdata.New(consumer)

	account_id := "DEBUGABCDEF123456"
	distinct_id := "DABCDEF123456"

	// 设置用户属性
	ta.UserSet(account_id, distinct_id, map[string]interface{}{
		"firstValue": 12345,
		"amount":     10,
	})

	// 设置一次用户属性，不覆盖同名属性
	ta.UserSetOnce(account_id, distinct_id, map[string]interface{}{
		"firstValue":        123457,
		"amount":            20,
		"modifiable_string": "hello",
	})

	// 数值型用户属性增加
	err = ta.UserAdd(account_id, distinct_id, map[string]interface{}{
		"amount": 50,
	})
	if err != nil {
		fmt.Println("user add failed", err)
	}

	// 设置公共事件属性
	ta.SetSuperProperties(map[string]interface{}{
		"super_string": "supervalue",
		"super_bool":   false,
	})

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

	// 记录用户浏览商品事件
	err = ta.Track(account_id, distinct_id, "ViewProduct", properties)
	if err != nil {
		fmt.Println(err)
	}

	// 清除公共事件属性
	ta.ClearSuperProperties()

	// 记录用户订单付款事件
	err = ta.Track(account_id, distinct_id, "PaidOrder", properties)
	if err != nil {
		fmt.Println(err)
	}

	ta.Flush()
	defer ta.Close()
}
