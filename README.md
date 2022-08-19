# Golang SDK 使用指南

本指南将会为您介绍如何使用 Golang SDK 接入您的项目。您可以在访问GitHub获取 Golang SDK 的源代码。

**最新版本为：** 1.6.6

**更新时间为：** 2022-08-19

## 一、集成并初始化 SDK
### 1.1 集成 SDK
执行以下命令获取最新版 Golang SDK。

```Shell
# 获取 SDK
go get github.com/ThinkingDataAnalytics/go-sdk/thinkingdata

# 更新 SDK
go get -u github.com/ThinkingDataAnalytics/go-sdk/thinkingdata

```
Module模式：

```Go
//在代码文件开头引入thinkingdata
import	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"

# 拉取最新SDK模块
go mod tidy
```

### 1.2 初始化 SDK
首先在代码文件开头引入 thinkingdata:

```Go
// import thinkingdata sdk
import	"github.com/ThinkingDataAnalytics/go-sdk/thinkingdata"

```
为使用 SDK 上传数据，需首先创建 TDAnalytics 实例. 创建 TDAnalytics 实例需要传入实现了 Consumer 接口的结构体. Consumer 的定义如下：

```Go
// Consumer 为数据实现 IO 操作（写入磁盘或者发送到接收端）
type Consumer interface {
	Add(d Data) error
	Flush() error
	Close() error
}

```

thinkingdata 包提供了 Consumer 的三种实现:

**(1) LogConsumer:** 将数据实时写入本地文件，文件以天/小时切分，并需要与 LogBus 搭配使用进行数据上传

```Go
// 创建按天切分的 LogConsumer, 不设置单个日志上限
consumer, err := thinkingdata.NewLogConsumer("/path/to/data", thinkingdata.ROTATE_DAILY)

// 创建按小时切分的 LogConsumer, 不设置单个日志上限
consumer, err := thinkingdata.NewLogConsumer("/path/to/data", thinkingdata.ROTATE_HOURLY)

// 创建按天切分的 LogConsumer，并设置单个日志文件上限为 10 G
consumer, err := thinkingdata.NewLogConsumerWithFileSize("/path/to/data", thinkingdata.ROTATE_DAILY, 10 * 1024)

// 指定生成的文件前缀
config := LogConfig{
		Directory:  "/path/to/data",
		RotateMode: thinkingdata.ROTATE_DAILY,
		FileNamePrefix: "prefix",
	}
consumer, err := thinkingdata.NewLogConsumerWithConfig(config)
```

传入的参数为写入本地的文件夹地址，您只需将 LogBus 的监听文件夹地址设置为此处的地址，即可使用 LogBus 进行数据的监听上传.

**(2) BatchConsumer:** 批量实时地向 TA 服务器传输数据，不需要搭配传输工具，在长时间网络中断情况下，有数据丢失的风险

```Go
// 创建 BatchConsumer, 指定接收端地址、APP ID
consumer, err := thinkingdata.NewBatchConsumer("SERVER_URL", "APP_ID")

// 创建 BatchConsumer, 设置数据不压缩，默认gzip压缩，可在内网传输
consumer, err := thinkingdata.NewBatchConsumerWithCompress("SERVER_URL", "APP_ID",false)

```

SERVER_URL为传输数据的 URL，APP_ID为您的项目的 APP ID

如果您使用的是云服务，请输入以下 URL:

http://receiver.ta.thinkingdata.cn

如果您使用的是私有化部署的版本，请输入以下 URL:

http://数据采集地址

注：1.1.0 版本之前输入以下 URL:

http://receiver.ta.thinkingdata.cn/logagent

http://数据采集地址/logagent

BatchConsumer 会先将数据存放在缓冲区中，当数据条数超过设定的值(batchSize, 默认为 20)，触发上报. 您也可以在创建 BatchConsumer 的时候指定 batchSize:

```Go
// 创建 BatchConsumer, 指定接收端地址、APP ID、缓冲区大小，单位为M
consumer, err := thinkingdata.NewBatchConsumerWithBatchSize("SERVER_URL", "APP_ID", 50)
```

**(3) DebugConsumer:** 逐条实时向 TA 服务器传输数据，当数据格式错误时会返回详细的错误信息。建议先使用 DebugConsumer 校验数据格式，不建议在生产环境中使用

```Go
consumer, _ := thinkingdata.NewDebugConsumer("SERVER_URL", "APP_ID")
```

如果您不想数据入库，只想校验数据格式，您可以初始化代码如下：

```Go
//默认是true，代表入库
consumer, _ := thinkingdata.NewDebugConsumerWithWriter("SERVER_URL", "APP_ID",false)
```

SERVER_URL为传输数据的 URL，APP_ID为您的项目的 APP ID

如果您使用的是云服务，请输入以下 URL:

http://receiver.ta.thinkingdata.cn

如果您使用的是私有化部署的版本，请输入以下 URL:

http://数据采集地址

### 1.3 创建 SDK 实例
传入创建好的 Consumer，即可得到对应的 TDAnalytics 实例:

```Go
ta, err := thinkingdata.New(consumer)
```

后续即可使用 ta 的接口来上报数据.

## 二、上报数据
在 SDK 初始化完成之后，您就可以调用 track 来上传事件，一般情况下，您可能需要上传十几到上百个不同的事件，如果您是第一次使用 TA 后台，我们推荐您先上传几个关键事件。

如果您对需要发送什么样的事件有疑惑，可以查看快速使用指南了解更多信息。

### 2.1 发送事件
您可以调用track来上传事件，建议您根据先前梳理的文档来设置事件的属性以及发送信息的条件：

```Go
// 设置事件属性
properties := map[string]interface{}{
    // 系统预置属性, 可选. "#time" 属性是系统预置属性，传入 time.Time 对象，也可以上传符合TA的时间字符串，表示事件发生的时间
    // 如果不填入该属性，则默认使用系统当前时间
    //"#time": time.Now().UTC(),
    //"#time":"2020-02-02 11:49:43.222",
    // 系统预置属性, 可选. 如果服务端中能获取用户 IP 地址, 并填入该属性
    // 数数会自动根据 IP 地址解析用户的省份、城市信息
    "#ip": "123.123.123.123",
    // 用户自定义属性, 字符串类型
    "prop_string": "abcdefg",
    // 用户自定义属性, 数值类型
    "prop_num": 56.56,
    // 用户自定义属性, bool 类型
    "prop_bool": true,
    // 用户自定义属性, time.Time 类型
	"prop_date": time.Now(),
}

account_id := "user_account_id" // 账号 ID
distinct_id := "ABCDEF123456AGDCDD" // 访客 ID

// 上报事件名为 TEST_EVENT 的事件. account_id 和 distinct_id 不能同时为空
ta.Track(account_id, distinct_id, "TEST_EVENT", properties)
```

注： 为了保证访客 ID 与账号 ID 能够顺利进行绑定，如果您的游戏中会用到访客 ID 与账号 ID，我们极力建议您同时上传这两个 ID，否则将会出现账号无法匹配的情况，导致用户重复计算，具体的 ID 绑定规则可参考用户识别规则一章。

- 事件的名称只能以字母开头，可包含数字，字母和下划线“_”，长度最大为 50 个字符，对字母大小写不敏感
- 事件的属性是 map 类型，其中每个元素代表一个属性
- 事件属性的 Key 值为属性的名称，为 string 类型，规定只能以字母开头，包含数字，字母和下划线“_”，长度最大为 50 个符，对字母大小写不敏感
- 事件属性的 Value 值为该属性的值，支持string、数值类型、bool、time.Time、数组类型、对象、对象组

### 2.2 设置公共事件属性
对于一些需要出现在所有事件中的属性属性，您可以调用SetSuperProperties来设置公共事件属性，我们推荐您在发送事件前，先设置公共事件属性。

```Go
// 设置公共事件属性
ta.SetSuperProperties(map[string]interface{}{
	"SUPER_TIME":   time.Now(),
	"SUPER_BOOL":   true,
	"SUPER_STRING": "hello",
	"SUPER_NUM":    15.6,
})
```

- 公共事件属性是 map 类型，其中每个元素代表一个属性
- 公共事件属性的 Key 值为属性的名称，为 string 类型，规定只能以字母开头，包含数字，字母和下划线“_”，长度最大为 50 个符，对字母大小写不敏感
- 公共事件属性的 Value 值为该属性的值，支持string、数值类型、bool、time.Time、数组类型、对象、对象组


设置公共属性，相当于给所有事件中都设置了上述属性，如果事件中的属性与公共属性重名，则该条数据的事件属性会覆盖同名的公共事件属性。 如果不存在同名属性，则添加该属性，可以通过接口获得当前公共事件属性:

```Go
currentSuperProperties := ta.GetSuperProperties()
```

您可以调用ClearSuperProperties清除所有已设置的公共属性。

```Go
ta.ClearSuperProperties()
```

### 2.3 设置动态公共事件属性
在 v1.6.0 版本中，新增了动态公共属性的特性，即公共属性可以上报时获取当时的值，使得诸如会员等级之类的可变公共属性可以被便捷地上报。调用 SetDynamicSuperProperties 函数，传入一个可以返回map类型的匿名函数。SDK 将会在事件上报时自动执行该匿名函数，并将返回值添加到触发的事件中。

动态公共事件属性的优先级会覆盖公共事件属性。

```Go
// 设置动态公共事件属性
ta.SetDynamicSuperProperties(func() map[string]interface{} {
    result := make(map[string]interface{})
    result["dynamic_super_time"] = time.Now()
    return result
})

// 清空动态公共事件属性
ta.SetDynamicSuperProperties(nil)
```


## 三、用户属性
TA 平台目前支持的用户属性设置接口为UserSet、UserSetOnce、UserAdd、UserDelete、UserUnset以及UserAppend。

### 3.1 UserSet
对于一般的用户属性，您可以调用 UserSet 来进行设置. 使用该接口上传的属性将会覆盖该用户原有的用户属性，如果之前不存在该用户属性，则会新建该用户属性:

```Go
ta.UserSet(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "some message",
	"USER_DATE":   time.Now(),
})

//再次上传用户属性，该用户的"USER_STRING"被覆盖为"another message"

ta.UserSet(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "another message"
})

```

属性格式要求与事件属性保持一致。

### 3.2 UserSetOnce
如果您希望上传的用户属性只被设置一次，则可以调用UserSetOnce来进行设置，当该属性之前已经有值的时候，将会忽略这条信息:

```Go
ta.UserSetOnce(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "some message",
	"USER_DATE":   time.Now(),
})
//再次上传用户属性，该用户的"USER_STRING"仍为"some message"

ta.UserSetOnce(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "another message"
})

//再次使用UserSet上传用户属性，该用户的"USER_STRING"会被覆盖为"other message"

ta.UserSet(account_id, distinct_id, map[string]interface{}{
	"USER_STRING": "other message"
})

```
属性格式要求与事件属性保持一致。

### 3.3 UserAdd
当您要上传数值型的属性时，您可以调用UserAdd来对该属性进行累加操作，如果该属性还未被设置，则会赋值 0 后再进行计算，可传入负值，等同于相减操作。

```Go
ta.UserAdd(account_id, distinct_id, map[string]interface{}{
	"Amount": 50,
})

//再次上传用户属性，该用户的"Amount"现为80

ta.UserAdd(account_id, distinct_id, map[string]interface{}{
	"Amount": 30,
})
```

设置的属性key为字符串，Value 只允许为数值。

### 3.4 UserDelete
如果您要删除某个用户，可以调用UserDelete将这名用户删除，您将无法再查询该名用户的用户属性，但该用户产生的事件仍然可以被查询到

```Go
ta.UserDelete(account_id, distinct_id)
```

### 3.5 UserUnset
当您需要清空某个用户的用户属性的值时，可以调用 UserUnset 进行清空：

```Go
// 清空某个用户的某个用户属性，在参数中传入属性名
ta.UserUnset(account_id, distinct_id, property_name)

```
UserUnset: 的传入值为被清空属性的 Key 值。

### 3.6 UserAppend
当您要为数组 类型追加用户属性值时，您可以调用UserAppend来对指定属性进行追加操作，如果该属性还未在集群中被创建，则UserAppend创建该属性

```Go
//用户数组类型追加属性 UserAppend ,为下面两个数组类型添加以下属性，只支持key - []string
err = ta.UserAppend(account_id, distinct_id, map[string]interface{}{
		"array": []string{"str1","str2"},
		"arrkey1":[]string{"str3","str4"},
	})
if err != nil {
	fmt.Println("user add failed", err)
}
```

### 3.7 UserUniqAppend
从 v1.6.0 版本开始，您可以调用 UserUniqAppend 对切片类型的用户属性进行追加操作。

和 UserAppend 的区别：调用 UserUniqAppend 会对追加的用户属性进行去重， UserAppend 接口不做去重，用户属性可存在重复。

```Go
ta.UserUniqAppend(account_id, distinct_id, map[string]interface{}{
    "array":   []string{"str1", "str2"},
    "arrkey1": []string{"str3", "str4"},
})
```

## 四、其他操作
###4.1 立即提交数据
此操作与具体的 Consumer 实现有关. 在收到数据时, Consumer 可以先将数据存放在缓冲区, 并在一定情况下触发真正的数据 IO 操作, 以提高整体性能. 在某些情况下需要立即提交数据，可以调用 Flush 接口

```Go
// 立即提交数据到相应的接收端
ta.Flush()

```

### 4.2 关闭 sdk
BatchConsumer 在服务器关闭或sdk退出前必须执行Close方法，否则可能会导致部分数据丢失。
```Go
// 关闭并退出 SDK
ta.Close()
```

关闭并退出 sdk，请在关闭服务器前调用本接口，以避免缓存内的数据丢失

## 五、相关预置属性
### 5.1 所有事件带有的预置属性
以下预置属性，是 Go SDK 中所有事件（包括自动采集事件）都会带有的预置属性

| 属性名 | 中文名 | 说明 |
| ----- | ----- | ----- |
| #ip | IP 地址 | 用户的 IP 地址，需要进行手动设置，TA 将以此获取用户的地理位置信息 |
| #country | 国家 | 用户所在国家，根据 IP 地址生成 |
| #country_code | 国家代码 | 用户所在国家的国家代码(ISO 3166-1 alpha-2，即两位大写英文字母)，根据 IP 地址生成 |
| #province | 省份 | 用户所在省份，根据 IP 地址生成 |
| #city | 城市 | 用户所在城市，根据 IP 地址生成 |
| #lib | SDK 类型 | 您接入 SDK 的类型，如 tga_go_sdk 等 |
| #lib_version | SDK 版本 | 您接入 Go SDK 的版本 |

## 六、进阶功能
从 v1.2.0 开始，SDK 支持上报两种特殊类型事件: 可更新事件、可重写事件。这两种事件需要配合 TA 系统 2.8 及之后的版本使用。由于特殊事件只在某些特定场景下适用，所以请在数数科技的客户成功和分析师的帮助下使用特殊事件上报数据。

### 6.1 可更新事件
您可以通过可更新事件实现特定场景下需要修改事件数据的需求。可更新事件需要指定标识该事件的 ID，并在创建可更新事件对象时传入。TA 后台将根据事件名和事件 ID 来确定需要更新的数据。

```Go
// 示例： 上报可被更新的事件，假设事件名为 UPDATABLE_EVENT
proterties := make(map[string]interface{})
properties["status"] = 3
properties["price"] = 100

consumer, _ := thinkingdata.NewBatchConsumer("url", "appid")
ta := thinkingdata.New(consumer)

// 上报后事件属性 status 为 3, price 为 100
ta.TrackUpdate("account_id", "distinct_id", "UPDATABLE_EVENT", "test_event_id", properties))

proterties_new := make(map[string]interface{})
proterties_new["status"] = 5

// 上报后事件属性 status 被更新为 5, price 不变
ta.TrackUpdate("account_id", "distinct_id", "UPDATABLE_EVENT", "test_event_id", proterties_new))

```

### 6.2 可重写事件
可重写事件与可更新事件类似，区别在于可重写事件会用最新的数据完全覆盖历史数据，从效果上看相当于删除前一条数据，并入库最新的数据。TA 后台将根据事件名和事件 ID 来确定需要更新的数据。

```Go
// 示例： 上报可被重写的事件，假设事件名为 OVERWRITE_EVENT
proterties := make(map[string]interface{})
properties["status"] = 3
properties["price"] = 100

consumer, _ := thinkingdata.NewBatchConsumer("url", "appid")
ta := thinkingdata.New(consumer)

// 上报后事件属性 status 为 3, price 为 100
ta.TrackOverwrite("account_id", "distinct_id", "OVERWRITE_EVENT", "test_event_id", properties))

proterties_new := make(map[string]interface{})
proterties_new["status"] = 5

// 上报后事件属性 status 被更新为 5, price 属性被删除
ta.TrackOverwrite("account_id", "distinct_id", "OVERWRITE_EVENT", "test_event_id", proterties_new))

```

### 6.3 首次事件
使用“首次事件校验”特性，必须要使用 TrackFirst 函数。firstCheckId 参数类型为string，该字段是校验首次事件的标识 ID，该 ID 首条出现的数据将入库，之后出现的都无法入库。不同事件的 firstCheckId 互相独立，因此每个事件的首次校验互不干扰。

```Go
// 设置事件属性
properties := map[string]interface{}{
    // 用户自定义属性, 字符串类型
    "prop_string": "abcdefg",
}
account_id := "user_account_id" // 账号 ID
distinct_id := "ABCDEF123456AGDCDD" // 访客 ID
// 首次事件
err := ta.TrackFirst(account_id, distinct_id, "EventName", "firstCheckId", properties)
```
