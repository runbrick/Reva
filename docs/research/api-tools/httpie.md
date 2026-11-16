# HTTPie CLI 对 Reva 的功能研究

> 最后一次上游取证：2026-11-16T22:29:59+08:00；首次抓取：2026-11-16T22:26:23.785885+08:00。Reva 基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；分类：`cli-test`。

## 结论与范围

HTTPie CLI 是免品牌账号的 Python 终端 HTTP 客户端，以可读请求语法、离线构造、持久会话、流式输出和 shell 组合见长；适合补充 Reva 的终端工作流。Reva 已有集合、变量、JavaScript、WebSocket 和本地 GUI，不能把这些现有功能当作待补缺口。

对象是规范仓库 [httpie/cli](https://github.com/httpie/cli)，不是同品牌桌面产品。官方文档及 GitHub latest release 均指向 **3.2.4，发布于 2024-11-01**。[1][4][16] 功能判断基于官方文档、实际源码与版本记录，不复制上游代码。

## 许可证、产品边界与架构

**CLI 为 BSD-3-Clause，属于 OSI 开源。** 已核对稳定版许可正文的三个条件，并与 GitHub SPDX 元数据和 setup.cfg 的 OSI BSD 分类交叉印证；允许商用和修改分发，须保留声明，且不得借作者名背书。[2][3] 这是开源许可，不是仅供查看的 source-available 许可。

README 指向 Desktop/Web 不代表其功能属于 CLI。另行读取的 HTTPie 服务条款 1.2、3、4 规定有限使用许可、专有权利与可能收费的服务；它们不替代 CLI 的 BSD 许可，也不足以判断当前每个套餐的收费边界。桌面集合、云工作区、团队协作及 OAuth 流程不计入本次 CLI 能力，第三方插件也不能统一标为 BSD。[1][21][22]

http/https 命令解析参数，以 Requests 发送 HTTP/TLS 请求，通过 requests-toolbelt、multidict 处理上传和多值结构；core 迭代请求/响应并选择标准流，Pygments/Rich 负责终端呈现。session 为本地 JSON；Python 插件扩展认证、传输、转换和格式化，httpie 管理命令与发送入口分离。Reva 则以 Go net/http、Viva、服务接口和本地 JSON 组成原生进程，GUI 通过 ui.Effect 发送。[3][17–20]

## 已实现的十项能力

| 能力 | 已实现行为及边界 | 主要证据 |
| --- | --- | --- |
| 按类型构建嵌套 JSON | key=value、key:=value 与方括号路径分别构造字符串、原生 JSON 值和嵌套数组/对象，支持从文件读入字段。3.0.0 changelog 明确记录嵌套语法；不必手写完整 JSON。 | [官方章节](https://httpie.io/docs/cli/nested-json) |
| 离线构造并输出请求 | --offline 构建并打印请求而不发网络请求，可重定向到 request.http；不是 mock 服务。显式输入仍可读取，使用 session 仍可能改变会话文件，不能理解成绝对无副作用。 | [官方章节](https://httpie.io/docs/cli/offline-mode) |
| 可复用的本地会话 | --session 持久化自定义头、认证及 Cookie，支持按主机命名或指定 JSON 路径。--session-read-only 对已有文件不回写，但文件不存在时仍会创建；凭据、密码和 Cookie 均为明文。 | [官方章节](https://httpie.io/docs/cli/sessions) |
| 逐行响应与 SSE 自动流式显示 | --stream 小块刷新并逐行格式化，text/event-stream 自动启用流式显示。这是 HTTP 流式输出，不等于已实现 WebSocket 或完整 SSE 重连管理。 | [官方章节](https://httpie.io/docs/cli/streamed-responses) |
| 下载、管道与续传 | --download 正文落盘并向 stderr 显示进度，--output 选择路径，--continue 使用 Range 续传；可边下载边输出到管道。续传需要服务器正确支持范围请求。 | [官方章节](https://httpie.io/docs/cli/download-mode) |
| 内置 Basic、Digest、Bearer 认证 | 内置三种认证，支持交互式密码输入和 netrc；额外认证经第三方插件扩展。Bearer 在 3.0.0 交付，不将第三方插件或桌面 OAuth 流程算作内置能力。 | [官方章节](https://httpie.io/docs/cli/authentication) |
| 代理及客户端 TLS 证书 | --proxy 可按协议指定代理，支持 SOCKS 与代理环境变量；可选 CA bundle、客户端证书/私钥和 --cert-key-pass。TLS 可用版本取决于 Python/OpenSSL 环境。 | [官方章节](https://httpie.io/docs/cli/client-side-ssl-certificate) |
| 终端格式化与交换元信息 | 支持 JSON/XML 格式化、颜色、选择输出请求/响应部分、显示重定向中间交换以及 --meta 总耗时；这些是排障观测，不是并发负载测试。 | [官方章节](https://httpie.io/docs/cli/response-meta) |
| shell/CI 退出码与标准流 | --check-status 将未跟随的 3xx、4xx、5xx 映射为退出码 3、4、5，超时为 2；自动化应显式加 --ignore-stdin。省略 --check-status 时收到 HTTP 500 不等同进程失败；这不证明内置集合 runner 或 JavaScript 生命周期。 | [官方章节](https://httpie.io/docs/cli/scripting) |
| 插件接口与管理器 | httpie cli plugins 支持安装、列出、升级、卸载；基类提供认证、传输、转换及格式化扩展。插件在 Python 进程中运行，依赖与许可证需单独审查，不是 Reva 能直接加载的 Go 插件。 | [官方章节](https://httpie.io/docs/cli/plugin-manager) |

以上均为已有能力。changelog 可交叉确认 2.0.0 的 offline、3.0.0 的嵌套 JSON/Bearer/SSE 自动流式/耗时及 3.1.0 的私钥口令。[15] 续传与代理见 [10][26]，退出状态映射见稳定版源码 [23][24]。

## 与当前 Reva 的直接比较

Reva 路径均相对于仓库根目录，标签中的行号针对上述基线。缺口同时检查数据模型、实际服务接线及功能文件，没有把 README 缺少说明视为功能不存在。

| 领域 | HTTPie CLI | 当前 Reva | 机会与判断 |
| --- | --- | --- | --- |
| 协议与交互 | 核心是 HTTP(S)；SSE 流式显示已实现，WebSocket 仍见开放提案 #1697；本次未核实内置 HTTP/2 或 gRPC。 | net/http 默认传输，未禁用默认 HTTPS HTTP/2 协商；另有 RFC 6455 ws/wss 客户端与 GraphQL 查询/变量编辑器。HTTP 响应完成后才进入界面，未见 SSE 增量显示。<br>证据：[internal/service/request/request.go:50](../../../internal/service/request/request.go)；[internal/service/request/options.go:41](../../../internal/service/request/options.go)；[internal/websocket/client.go:34](../../../internal/websocket/client.go)；[internal/gui/graphql.go:48](../../../internal/gui/graphql.go)；[internal/gui/app.go:261](../../../internal/gui/app.go) | 保留现有 WebSocket/GraphQL 优势，补 SSE 增量显示；两者 HTTP/2 协商未实测。 |
| 认证与传输选项 | 内置 Basic/Digest/Bearer、netrc、客户端证书及自定义 CA；额外认证依赖插件，不是完整 OAuth2 生命周期。 | Basic/Bearer/API-key，集合/文件夹继承与显式 None 屏障；跨源重定向保护认证头；可设超时、跳转和 TLS 校验。模型未见 Digest、netrc、客户端证书或 CA 路径字段。<br>证据：[internal/model/model.go:92](../../../internal/model/model.go)；[internal/auth/auth.go:35](../../../internal/auth/auth.go)；[internal/service/request/request.go:184](../../../internal/service/request/request.go)；[internal/service/request/options.go:41](../../../internal/service/request/options.go) | 复杂 mTLS/代理场景先明确外部工具路线，再按真实需求扩展模型。 |
| 变量与会话 | shell 变量在进入 CLI 前展开；session 保存头、认证和 Cookie，不是 Postman 环境作用域。 | {{name}} 覆盖 URL、参数、头、正文、文件路径和认证字段；环境覆盖集合变量，未知变量保留。HTTP client 未配置 Jar，不能自动跨发送复用 Cookie。<br>证据：[internal/vars/vars.go:14](../../../internal/vars/vars.go)；[internal/vars/vars.go:94](../../../internal/vars/vars.go)；[internal/gui/env.go:42](../../../internal/gui/env.go)；[internal/service/request/options.go:68](../../../internal/service/request/options.go) | 增加显式启用且可清空的 Cookie 会话，不将会话存储冒充变量系统。 |
| 脚本与断言 | 官方 scripting 指 shell 编排与退出码；在已读 CLI 文档和定义中未见 Postman 风格前置/响应 JavaScript API。 | Otto 隔离 JavaScript，默认 2 秒限时；RunPre 修改本次请求/变量，RunTests 提供 pm.test、pm.expect、pm.response；不支持异步测试，变量修改不写回持久环境。<br>证据：[internal/script/script.go:17](../../../internal/script/script.go)；[internal/script/script.go:61](../../../internal/script/script.go)；[internal/script/api.js:94](../../../internal/script/api.js)；[internal/gui/scripts.go:59](../../../internal/gui/scripts.go) | 复用 Reva 已有脚本，不把 shell scripting 当作待补 JS 功能。 |
| 批量 runner 与 CI | shell、管道与 --check-status 编排单次请求；未核实内置集合 runner、数据驱动矩阵或 JUnit 报告。 | cmd/reva 启动 GUI，接口提供单请求 Send；当前入口、源码树和服务中未见集合 runner、无界面执行或 CI 测试报告导出。<br>证据：[cmd/reva/main.go:12](../../../cmd/reva/main.go)；[internal/service/service.go:20](../../../internal/service/service.go)；[internal/gui/app.go:225](../../../internal/gui/app.go) | 简单探活直接交给 HTTPie shell；未来 Reva runner 应另立契约并复用现有脚本。 |
| 导入导出与迁移 | 接受命令参数、stdin/文件与 session JSON，--offline 输出请求文本；未见原生 Postman/HAR/OpenAPI 集合导入器。 | 实际 importer 支持 Reva、Postman、HAR、curl、Swagger/OpenAPI JSON；有审阅和告警、集合 JSON 导出、复制 POSIX curl、加密备份。OpenAPI 转换的是模板。<br>证据：[internal/importer/result.go:57](../../../internal/importer/result.go)；[internal/gui/importer.go:587](../../../internal/gui/importer.go)；[internal/gui/collections.go:870](../../../internal/gui/collections.go)；[internal/gui/request.go:659](../../../internal/gui/request.go)；[internal/gui/sync.go:274](../../../internal/gui/sync.go) | 现有 curl 可连接外部工具；session JSON 不能视为集合的无损交换格式。 |
| Mock | --offline 仅构造请求；已读 CLI 文档、入口和 client.py 未见提供端点的 mock 服务器。 | 生产服务接口与 cmd/reva 未见 mock 服务器入口；service/fake 是测试替身。<br>证据：[internal/service/service.go:20](../../../internal/service/service.go)；[cmd/reva/main.go:12](../../../cmd/reva/main.go)；[internal/service/fake/fake.go](../../../internal/service/fake/fake.go) | 需要 mock 时接专门的本地服务，不把 fake 或预览称为用户 mock。 |
| Schema 验证 | JSON 支持是构造与格式化；稳定版文档及 CLI 定义未见 OpenAPI/JSON Schema 响应校验命令。 | OpenAPI sample 明确生成模板而不验证 schema 约束；脚本有通用断言，但无完整 schema 验证器。<br>证据：[internal/importer/openapi.go:784](../../../internal/importer/openapi.go)；[internal/script/api.js:54](../../../internal/script/api.js) | 契约验证适合接专业 schema 测试器，保留本地规范文件边界。 |
| 性能与大数据 | 流式上传/响应/下载，--meta 是单次耗时，不提供已核实的并发吞吐或百分位负载报告。 | 二进制及 multipart 文件从磁盘流式读；响应默认保留 10 MiB，其余仍读取并丢弃以统计完整大小；有 DNS/TCP/TLS/发送/等待/下载时间线。<br>证据：[internal/service/request/body.go:19](../../../internal/service/request/body.go)；[internal/service/request/request.go:24](../../../internal/service/request/request.go)；[internal/service/request/request.go:135](../../../internal/service/request/request.go)；[internal/service/request/trace.go](../../../internal/service/request/trace.go) | 补增量显示及及时取消，不重写已有流式上传，也不将任一项目定位成压测器。 |
| 协作与同步 | 脚本和 JSON 可存 Git；CLI 未见账号、团队空间、权限或云同步，同品牌桌面/云服务有独立专有服务条款。 | 本地免账号；已有 AES-GCM 加密快照、WebDAV/S3/Gist 存储抽象和冲突检测，不是实时团队编辑或 RBAC。<br>证据：[internal/storage/storage.go:27](../../../internal/storage/storage.go)；[internal/syncing/sync.go:49](../../../internal/syncing/sync.go)；[internal/cloudsync/cloudsync.go:1](../../../internal/cloudsync/cloudsync.go) | 继续使用用户自选存储，不为 CLI 比较引入账号体系。 |
| 本地数据与凭据 | session 是可编辑的明文 JSON；--session-read-only 仍可能创建文件，普通 CLI 请求无需品牌账号。 | 集合、环境、历史、设置均为本地 JSON；REVA_HOME 可覆盖目录。输入遮罩不是保险库，可选加密备份不改变本地 JSON 属性。<br>证据：[internal/storage/storage.go:22](../../../internal/storage/storage.go)；[internal/model/model.go:310](../../../internal/model/model.go)；[internal/syncing/sync.go:49](../../../internal/syncing/sync.go)；[README.md](../../../README.md) | 增加会话前明确临时/持久行为，避免把本地 JSON 描述成加密凭据库。 |

## 六个功能议题：状态与交付分开

先获取一次按更新时间排序的 100 条 issue/PR 混合列表，再结合 changelog 读取单议题；实际为 PR 的 #1169 已排除。以下 **2 个开放提案、4 个有版本交付证据的功能议题** 均核实为 issue，排除了普通兼容性 bug 和例行维护。`closed` 只表示追踪状态，交付另查；本样本没有用关闭/重复而无交付证明的项目冒充成果。

| 议题原题 | 状态 / 标签 / 更新时间 | 用户需求 | 交付判定与证据 |
| --- | --- | --- | --- |
| [#1697 Basic WebSocket Client Support in CLI](https://github.com/httpie/cli/issues/1697) | `open` / enhancement, new / `2026-08-12T11:33:40Z` | 终端用户想在 HTTP 调试后直接发送一个 WebSocket 文本帧并读取响应，减少切换工具；提案明确不含 REPL 和持续流。 | **开放提案，未核实交付**。https://github.com/httpie/cli/issues/1697；稳定版 CLI 文档未列 WebSocket；Reva 已有对应协议客户端。 |
| [#1425 Support httpie in Chrome DevTools](https://github.com/httpie/cli/issues/1425) | `open` / enhancement / `2026-09-13T14:34:49Z` | 希望 Chrome DevTools 直接复制为 HTTPie 命令，解决浏览器与终端之间的入口断点。 | **开放提案，未核实交付**。https://github.com/httpie/cli/issues/1425；原文请求社区浏览器扩展，不是 CLI 已交付的浏览器集成。 |
| [#1215 Support Bearer Authentication](https://github.com/httpie/cli/issues/1215) | `closed` / enhancement, new / `2021-12-29T09:09:08Z` | 把常见 Bearer token 变成显式认证选项，减少手写 Authorization；并不要求获取或刷新 OAuth token。 | **已验证交付：3.0.0**。https://github.com/httpie/cli/blob/master/CHANGELOG.md（3.0.0 明确引用 #1215）；https://httpie.io/docs/cli/bearer-auth |
| [#946 Support for client side certificate/key password](https://github.com/httpie/cli/issues/946) | `closed` / enhancement / `2022-03-07T13:09:07Z` | 自动化需要为加密客户端私钥提供口令，不能依赖终端交互；这是 mTLS 脚本化的具体需求。 | **已验证交付：3.1.0**。https://github.com/httpie/cli/blob/master/CHANGELOG.md（3.1.0 明确引用 #946）；https://httpie.io/docs/cli/client-side-ssl-certificate；client.py 使用 HTTPieCertificate。 |
| [#376 Server-sent events explicitly requires --stream](https://github.com/httpie/cli/issues/376) | `closed` / enhancement / `2021-12-08T15:49:13Z` | 长连接用户需要立刻看到 SSE 数据；按 Content-Type 自动切换比隐藏的手动开关更方便。 | **已验证交付：3.0.0**。https://github.com/httpie/cli/blob/master/CHANGELOG.md（3.0.0 明确引用 #376）；https://httpie.io/docs/cli/streamed-responses 明确自动启用 text/event-stream。 |
| [#243 Response time](https://github.com/httpie/cli/issues/243) | `closed` / enhancement, planned / `2024-11-29T21:47:14Z` | 用户想在终端观察单次请求耗时、判断延迟；这与并发压力测试属于不同需求。 | **已验证交付：3.0.0**。https://github.com/httpie/cli/blob/master/CHANGELOG.md（3.0.0 明确引用 #243，--meta/-vv/--print=m）；https://httpie.io/docs/cli/response-meta |

## 给 Reva 的三个具体改进

均保持本地、免账号、Go/Viva 方向；目前没有证据将这些增量定为阻断发布的 P0。建议先做预览，再处理会话和流式接口；下列是后续功能 PR 的范围，本研究没有修改对应代码。

### 1. 增加安全的离线请求预览（P1）

**场景：** 切换环境或继承集合认证后，开发者想先看最终 URL、认证来源与正文再发送。

**现有缺口：** 已有复制 curl 能展开变量并应用认证，但没有独立的可视预览契约；真实 Send 还运行前置脚本。

**最小 PR：** 新增只读预览动作，复用 effectiveAuth、VarsFor、vars.Apply 与认证逻辑，显示默认遮罩的 URL/头、正文摘要、未解析变量和传输选项；明确标注未运行前置脚本。文件仅展示路径，不打开文件；multipart 展示语义，不声称精确 wire bytes；不写历史、环境或集合。

**依赖：** 后续独立 PR 涉及 gui/request.go、共享认证/变量构造辅助和 i18n；沿用 Viva 弹窗与主题 token，不新增依赖或修改数据 schema。

**有意义的验收：** headless 测试创建继承 Bearer 的集合及覆盖 baseUrl 的环境，预览显示正确 URL、认证来源和遮罩；放入会抛错的前置脚本和不存在的上传路径仍可预览且脚本不执行；假 sender 被调用即失败，历史及数据文件保持不变。

### 2. 增加默认关闭的临时 Cookie 会话（P1）

**场景：** 先调本地登录接口，再调受保护资源，希望自动复用 Set-Cookie 并能清空登录态。

**现有缺口：** 目前只解析并显示 Cookie，HTTP client 无 Jar；认证继承不能代替服务端会话。

**最小 PR：** 仅做内存会话、显式启用及清空，用标准库 net/http/cookiejar。会话按明确作用域隔离，保留域/路径/Secure/过期规则；定义显式 Cookie 优先级及跨源重定向策略，不把 Jar 塞进跨请求共享 client 导致串用；本 PR 不做落盘。

**依赖：** 后续协调 RequestService 会话标识或实例隔离、service/request/options.go 的 client 缓存策略、GUI/i18n；复用 context 取消和重定向保护，不新增依赖或账号。

**有意义的验收：** httptest 登录设置 HttpOnly、Path、Max-Age Cookie，启用后匹配路径请求携带；不同会话、关闭或清空后不携带；验证过期/路径不匹配、跨主机跳转不泄漏，临时数据目录无新增凭据文件。

### 3. 为 SSE 增加可取消的增量文本显示（P1）

**场景：** 调试本地 AI token 流或任务进度，希望连接未结束时就看到第一段响应。

**现有缺口：** Send 在正文结束后才返回界面，10 MiB 保留上限也不等于流式 UI；HTTPie #376 提供了具体用户需求。

**最小 PR：** 增加可选响应片段回调或通道契约，按 Content-Type 识别 SSE，以有界批次发布；GUI 通过 Viva 支持的 Effect 状态通知消费，显示纯文本增量及停止按钮，保持有界缓存和截断提示。先不做 Last-Event-ID、自动重连或完整事件管理，禁止 GUI 自建 goroutine/轮询定时器。

**依赖：** 后续需共同修改 service.go、service/request、app.go 生命周期、response.go 与 i18n；先核实 Viva v0.6.0 的 Effect 通知 API，现有单次结果接口不足，不能承诺只改 response.go。

**有意义的验收：** httptest 服务 flush 第一段后等待测试控制信号，headless 界面须在结束信号前显示第一段；停止应取消服务器上下文并停止追加；超过缓冲上限显示截断且内存有界，普通 JSON 和历史记录无退化。

## 专长工具的集成边界

简单终端探活和 shell/CI 编排直接使用 HTTPie 的 `--check-status --ignore-stdin`；下载续传、复杂代理和客户端证书调试也可先交给 HTTPie。第三方认证插件由用户自行审查、选择和安装，Reva 无须嵌入 Python 或自动执行导入命令。已有复制 curl 能服务现有工具链；HTTPie 文档相关项目章节列出的第三方 curl→HTTPie 转换器未在本次验证，不能承诺无损转换。Reva 的 pm.test 不会因此自动获得 HTTPie 执行能力。

完整负载测试、OpenAPI 契约测试、mock 服务器宜使用专业工具，围绕本地请求或规范文件协作。是否增加 HTTPie 命令导出可另做小 PR，须明确 shell 转义、脚本、文件路径、传输选项的保真边界。[4][12–14]

## 验证与本地 PR 交付

仅新增 `docs/research/api-tools/httpie.md` 与 `httpie.json`，不新增行为测试。已读 AGENTS.md、README.md、docs/onboarding-prs.md；任务说明确认 TASK.md 与本地 Viva skill 缺失，本次不改 GUI。所有检查使用已有依赖并禁止 Go 模块网络下载，以下为本工作树真实退出码与输出尾部。

```text
$ gofmt -l .
（空输出）
退出码：0
```

```text
$ go vet ./...
（空输出）
退出码：0
```

```text
$ go tool viva vet ./...
（空输出）
退出码：0
```

```text
$ go test ./... -count=1
ok  	github.com/runbrick/reva/internal/model	1.454s
ok  	github.com/runbrick/reva/internal/s3	2.060s
ok  	github.com/runbrick/reva/internal/script	1.464s
?   	github.com/runbrick/reva/internal/service	[no test files]
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	1.665s
ok  	github.com/runbrick/reva/internal/service/store	1.495s
ok  	github.com/runbrick/reva/internal/storage	0.853s
ok  	github.com/runbrick/reva/internal/syncing	2.213s
ok  	github.com/runbrick/reva/internal/update	1.435s
ok  	github.com/runbrick/reva/internal/vars	1.093s
ok  	github.com/runbrick/reva/internal/websocket	0.054s
退出码：0
```

实际工具链：`go1.27.3 windows/amd64`，仓库目标为 Go 1.26。仅检查进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`。未安装依赖、未让测试访问互联网、未推送或合并。提交前另验证 JSON 结构、Markdown 一致性、UTF-8 汉字完整性和 `git diff --check`。

## 局限

- 实际观察日期为 2026-11-16；检索时间保留机器 +08:00 时区，议题 updated_at 保留 UTC，未用提示中的时区推算机器日期。网页及 API 均为抓取时快照，后续状态可能变化。
- 仅一次获取按更新时间排序的 all-state 100 条 issue/PR 混合列表，随后按列表或 changelog 直取单议题；未穷举历史。#1169 实际为 PR，未计入样本；六个样本都是 issue，含两个 open 与四个 closed。
- closed 不作为交付证据；四个关闭功能议题都有具体版本 changelog 和现行文档佐证。未将关闭/重复但无交付证明的项目包装为成果；开放提案明确标为未核实。
- 最新发布 API 与官方文档均指向 3.2.4（2024-11-01），不能仅凭发布日期判断项目停止维护；实现阅读也包含 master，来源区分稳定版与动态源码 URL。
- 未安装或运行 HTTPie、第三方插件或桌面产品，未执行线上 API 或性能基准。协议、TLS、性能与界面体验只作源码/文档判断，不承诺插件质量或桌面付费档位。
- 专有服务条款用于划分产品边界，不覆盖或收回 CLI 的 BSD 权利；第三方插件、Desktop/Web 不统一标为 BSD，也不将 source-available 混同 OSI 开源。
- Reva 缺口依据基线 model、实际入口、服务、GUI、importer/script 交叉阅读；未见表示本次范围没有实现证据，不断言所有分支或隐藏扩展都不存在。
- 本 PR 只改文档，不新增行为测试、不改源码或安装依赖。检查设置 GOPROXY=off、GOFLAGS=-p=2、GOMAXPROCS=2；实际为 Go 1.27.3，仓库目标 Go 1.26，本地通过不能替代 Go 1.26/macOS CI。

## 编号证据来源

1. [规范仓库、仓库元数据和 README](https://github.com/httpie/cli)
2. [稳定版 BSD 三条款许可正文](https://github.com/httpie/cli/blob/3.2.4/LICENSE)
3. [Python 依赖、OSI BSD 分类及命令入口](https://github.com/httpie/cli/blob/3.2.4/setup.cfg)
4. [官方 CLI 3.2.4 全文文档；一次抓取涵盖下列章节](https://httpie.io/docs/cli)
5. [嵌套 JSON 语法](https://httpie.io/docs/cli/nested-json)
6. [离线构造与会话副作用边界](https://httpie.io/docs/cli/offline-mode)
7. [本地会话、明文凭据和只读选项](https://httpie.io/docs/cli/sessions)
8. [逐行显示与 SSE 自动流式输出](https://httpie.io/docs/cli/streamed-responses)
9. [下载模式](https://httpie.io/docs/cli/download-mode)
10. [范围请求续传](https://httpie.io/docs/cli/resuming-downloads)
11. [内置认证与插件边界](https://httpie.io/docs/cli/authentication)
12. [客户端证书和私钥口令](https://httpie.io/docs/cli/client-side-ssl-certificate)
13. [shell 自动化与退出码](https://httpie.io/docs/cli/scripting)
14. [CLI 插件管理器](https://httpie.io/docs/cli/plugin-manager)
15. [历史版本交付记录；本次核对至 3.2.4](https://github.com/httpie/cli/blob/master/CHANGELOG.md)
16. [最新发布页：2024-11-01 发布的 3.2.4](https://github.com/httpie/cli/releases/tag/3.2.4)
17. [请求构造、离线分支、Requests 和 TLS 实现](https://github.com/httpie/cli/blob/master/httpie/client.py)
18. [标准流、响应迭代与退出状态实现](https://github.com/httpie/cli/blob/master/httpie/core.py)
19. [JSON 会话和 CookieJar 实现](https://github.com/httpie/cli/blob/master/httpie/sessions.py)
20. [认证、传输、转换、格式化插件基类](https://github.com/httpie/cli/blob/master/httpie/plugins/base.py)
21. [条款 1.2、3、4：有限许可、专有权利及付费服务边界](https://httpie.io/terms)
22. [同品牌桌面/Web 产品入口，不用于推导 CLI 功能](https://httpie.io/product)
23. [稳定版退出状态与 HTTP 状态映射](https://github.com/httpie/cli/blob/3.2.4/httpie/status.py)
24. [稳定版 CLI 参数定义](https://github.com/httpie/cli/blob/3.2.4/httpie/cli/definition.py)
25. [请求耗时元信息](https://httpie.io/docs/cli/response-meta)
26. [代理、环境变量与 SOCKS](https://httpie.io/docs/cli/proxies)
27. [功能议题 #1697：Basic WebSocket Client Support in CLI](https://github.com/httpie/cli/issues/1697)
28. [功能议题 #1425：Support httpie in Chrome DevTools](https://github.com/httpie/cli/issues/1425)
29. [功能议题 #1215：Support Bearer Authentication](https://github.com/httpie/cli/issues/1215)
30. [功能议题 #946：Support for client side certificate/key password](https://github.com/httpie/cli/issues/946)
31. [功能议题 #376：Server-sent events explicitly requires --stream](https://github.com/httpie/cli/issues/376)
32. [功能议题 #243：Response time](https://github.com/httpie/cli/issues/243)
