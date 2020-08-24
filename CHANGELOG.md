**v1.2.0** (2020/08/24)
- 支持track_update和track_overwrite接口

**v1.1.1** (2020/07/08)
- 字段#time支持上传符合TA格式的字符串
- 去除字段2k大小的限制

**v1.1.0** (2020/02/12)
- 支持上报数组类型
- 支持 UserAppend 接口
- DebugConsumer 优化: 在服务端对数据进行更完备准确地校验
- BatchConsumer 性能优化：支持配置压缩模式；移除 Base64 编码

**v1.0.2** (2019/12/25)
- 支持 UserUnset 接口

**v1.0.1** (2019/12/12)
- BugFix: 允许为空的属性值不写入日志中

**v1.0.0** (2019/09/25)
- 初始版本, 实现了数据上报核心功能
    - Track: 追踪用户行为事件
    - 公共事件属性设置
    - 用户属性设置: UserSet、UserSetOnce、UserAdd、UserDelete
- 支持: LogConsumer, DebugConsumer, BatchConsumer
