## OpenObserve

#### 本库是一个用于日志收集的 Go 语言组件库，支持异步和同步发送日志数据到指定的[OpenObserve](https://github.com/openobserve/openobserve)。该库提供了灵活的配置选项，包括基本认证、授权、请求超时、索引名称等。

## 安装

### 使用以下命令可以安装组件库：

```go
go get -u github.com/iamxvbaba/openobserve
```

## 使用示例

以下是一个简单的使用示例：

```go
package main

import (
    "context"
    "github.com/iamxvbaba/openobserve"
    "time"
)

func main() {
    // 创建一个上下文
    ctx := context.Background()

    // 创建 OpenObLog 实例
    log := openobserve.New(ctx, "http://localhost:5080", 
        openobserve.WithFullSize(100),
        openobserve.WithRequestTimeout(time.Second * 5),
        openobserve.WithIndexName("logs", true),
        openobserve.WithAuthorization("token"))

    // 发送日志数据
    log.Send(map[string]any{
			"_timestamp": time.Now().UnixMicro(),
			"name":       "xuhui",
			"message":    "你好",
			"log":        "ip-10-2-56-221.us-east-2.compute.internal",
			"i":          i,
		})
}

```

## 配置选项

提供了以下配置选项：

- `WithBasicAuth`: 使用用户名和密码进行基本认证。
- `WithAuthorization`: 使用 token 进行授权。
- `WithFullSize`: 指定达到的数量时马上请求。
- `WithWaitTime`: 每过指定时间发起请求。
- `WithRequestTimeout`: 设置 HTTP 请求超时时间。
- `WithIndexName`: 设置索引名称，并指定是否按天切割索引。

## 方法

提供了以下方法：

- `SendSync`: 同步发送日志数据。
- `Send`: 异步发送日志数据。

## 注意事项

- 使用前请确保你的日志服务地址和认证信息已正确配置。
- 请根据实际需求设置合适的配置选项和参数。