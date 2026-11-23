# Reva 本地接口测试服务

在仓库根目录运行：

```sh
go run ./cmd/reva-test-service
```

服务监听 `http://127.0.0.1:8787`。在 Reva 中选择「导入」，打开 [reva-test.postman_collection.json](reva-test.postman_collection.json)，确认后在集合里发送「健康检查」，应得到 `200` 和 `{"status":"ok"}`。导入中的本地文件引用提示对应两条上传请求。

如果端口被占用，可以运行 `go run ./cmd/reva-test-service -port 18787`，并把集合变量 `baseUrl` 改为 `http://127.0.0.1:18787`。在启动服务的终端按 `Ctrl+C` 可停止服务。

## 怎么测

| 场景 | 请求 | 预期结果 |
| --- | --- | --- |
| 登录 | `POST /auth/login`，JSON `{"username":"demo","password":"reva123"}` | `200`，响应包含 `access_token` 和 `Set-Cookie`。密码错误返回 `401` |
| Bearer 鉴权 | `GET /auth/bearer`，在认证面板填入登录返回的 `access_token` | `200`。清除或改错令牌后返回 `401` |
| Cookie 会话 | 在请求设置启用「使用临时 Cookie 会话」，依次发「尚未登录」「登录」「Cookie 会话」 | 清空会话后依次得到 `401`、`200`、`200`。三个请求使用同一集合和环境 |
| Basic / API Key | `GET /auth/basic` 用登录账号密码。`GET /auth/key` 用 `X-API-Key` 请求头或 `api_key` 查询参数 | 集合已填好测试值，成功返回 `200`，错误值返回 `401` |
| 传参 | 发送「路径、查询和请求头」「POST JSON」「PUT 表单」「PATCH XML」 | 检查中文路径、同名参数顺序、空值、`+`、请求头和相应的正文回显 |
| 请求方法 | `GET`、`POST`、`PUT`、`PATCH`、`DELETE`、`HEAD`、`OPTIONS` 请求 `/echo/{id}` | `HEAD` 有 `X-Echo-Id` 且无正文。`OPTIONS` 返回 `204` 和 `Allow`。其他方法回显收到的数据 |
| 上传 | `POST /upload` 用 `form-data`，也可加多个文件字段。`POST /upload/binary` 用 `Binary` | 在两条请求的 Body 面板分别重选一个小文件，可以选本说明文件。同一文件返回相同的字节数和 SHA-256。请求体上限为 8 MiB，含 multipart 封装 |
| 下载 | `GET /download` | `200`，附件名 `reva-sample.txt`。加请求头 `Range: bytes=0-3` 后返回 `206` 和 `Reva` |
| 响应格式 | `GET /formats/{json,xml,html,text,csv,gzip}` | 对比响应的 `Content-Type`、中文内容和 gzip 解压后的 JSON |
| 事件流 | `GET /events` | 先收到两条 `progress` 事件，再收到一条 `done` 事件并结束 |
| 状态码 | `GET /status/{204,400,404,429,500}` | `204` 无正文。`429` 另有 `Retry-After: 2` |
| 重定向 | `GET /redirect` | 开启跟随时最终为 `/health` 的 `200`。关闭跟随后显示 `302` 和 `Location: /health` |
| 超时 / 取消 | `GET /delay?ms=1500` | 正常约 1.5 秒返回 `200`。将请求超时改为 `300` 毫秒可触发超时，也可以将 `ms` 改为 `5000` 后取消 |

账号、令牌和 API Key 都是固定的演示值。上传接口读取文件并计算摘要，服务只监听本机回环地址。
