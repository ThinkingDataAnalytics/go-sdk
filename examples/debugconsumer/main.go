package main

import (
	"fmt"
	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"
	"time"
)

func main() {
	var err error

	// 创建 Debug Consumer
	consumer, _ := thinkingdata.NewDebugConsumer("url", "appid")
	//是否写入TA库，默认写入
	//consumer, _ := thinkingdata.NewDebugConsumerWithWriter("url", "appid",true)
	ta := thinkingdata.New(consumer)

	account_id := "DEBUGABCDEF123456"
	distinct_id := "DABCDEF123456"

	// 设置用户属性
	err = ta.UserSet(account_id, distinct_id, map[string]interface{}{
		"firstValue": 12345,
		"amount":     10,
		"#uuid":      "f5394eef-e576-4709-9e4b-a7c231bd34a4", //如果上传#uuid， 只支持UUID标准格式xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx的string类型
		"arr":        []string{"11", "12", "3333"},
	})

	if err != nil {
		fmt.Println(err)
	}
	//设置一次用户属性，不覆盖同名属性
	err = ta.UserSetOnce(account_id, distinct_id, map[string]interface{}{
		"firstValue":        123457,
		"amount":            20,
		"modifiable_string": "hello",
	})
	if err != nil {
		fmt.Println(err)
	}
	// 删除用户属性
	err = ta.UserUnset(account_id, distinct_id, []string{
		"modifiable_string",
	})
	if err != nil {
		fmt.Println(err)
	}
	// 数值型用户属性增加
	err = ta.UserAdd(account_id, distinct_id, map[string]interface{}{
		"amount": 50,
	})
	if err != nil {
		fmt.Println("user add failed", err)
	}
    //用户数组类型追加属性 UserAppend ,为下面两个数组类型添加以下属性，只支持key - []string
    err = ta.UserAppend(account_id, distinct_id, map[string]interface{}{
    		"array": []string{"str1","str2"},
    		"arrkey1":[]string{"str3","str4"},
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
		"#ip":     "123.123.123.123",
		"id":      "123",
		"catalog": "d",
		"is_bool": true,
	}

	// 记录用户浏览商品事件
	err = ta.Track(account_id, distinct_id, "ViewProduct", properties)
	if err != nil {
		fmt.Println(err)
	}

	// 清除公共事件属性
	ta.ClearSuperProperties()

	// track 事件
	err = ta.Track(account_id, distinct_id, "view_page", properties)
	if err != nil {
		fmt.Println(err)
	}

	ta.Flush()
	defer ta.Close()
}
