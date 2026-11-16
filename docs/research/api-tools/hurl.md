# Hurl 对照研究：让可审阅的 HTTP 会话进入 CI

Hurl 最值得 Reva 借鉴的是**响应捕获形成会话、文本测试便于审阅、执行结果能够直接进入 CI**。适合的产品关系是 Reva 负责本地交互探索、Hurl 承担专业会话测试；Reva 已有 JavaScript 脚本、GraphQL、WebSocket、原生导出和加密同步，不能把这些现有能力写成待补功能。

- 项目：**Hurl**；分类：`cli-test`；规范仓库：[Orange-OpenSource/hurl][1]。
- 实际检索结束：**2026-11-16T14:30:38+00:00**，即纽约时间 2026-11-16 09:30:38；不以模型知识截止日期代替抓取日期。
- 抓取时最新正式版：**8.1.0**，GitHub 发布时间 **2026-10-28T12:39:02Z**，版本说明标题日期为 2026-10-27。[3] [26]
- Reva 基线：`git rev-parse HEAD` 得到 **`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`**；本报告所有“当前 Reva”判断均针对该提交。
- 本地评审分支：`runbrick/research-hurl`；文档提交可用 `git log -1 --format=%H -- docs/research/api-tools/hurl.md docs/research/api-tools/hurl.json` 定位，避免在提交正文中自引用哈希。
- 配套结构化数据：[hurl.json](hurl.json)。本地拟议 PR 仅含这两份文档；未创建远端 PR、推送或合并。

## 定位、架构及许可边界

Hurl 用 `.hurl` 纯文本描述 HTTP 请求、期待响应、捕获与断言。文件既是可运行的测试，也是能随应用代码提交 Git 的会话说明。REST、SOAP、GraphQL 是通过 HTTP 请求正文支持的用途；不能由此推断它拥有独立的 gRPC 或 WebSocket 产品能力。[4]

源码的工作区分为 `hurl_core`（语法树、解析、格式化、类型）、`hurl`（HTTP、执行、断言、捕获、报告）和 `hurlfmt`（检查、格式化、转换）。HTTP 客户端经 Rust 绑定调用 **libcurl**，不是把文件翻译后启动 `curl` 子进程；一个文件内的请求按顺序执行，测试模式在文件之间并行。Rust 库公开 `runner::run`。构建仍涉及 libcurl、TLS、libxml2 等原生库，“独立命令行二进制”不表示所有平台均无系统依赖。[4] [5] [6] [7] [8] [9] [10]

主仓库固定标签中的许可正文为 **Apache-2.0，属于 OSI 批准的开源许可证**；GitHub 仓库许可元数据一致。未发现所研究的 CLI、捕获、认证、报告功能需要商业版授权，它们均有公开实现；本报告不把第三方服务计入社区版，也不宣称已审计所有商业合同。各依赖和发布所含原生库仍有各自许可，不能将 Apache-2.0 推广为整个依赖闭包的许可。[2] [27]

直接把 Rust/libcurl 嵌入 Go/Viva 会扩大构建与分发成本。更符合 Reva 无账户、本地优先方向的入口，是用户审阅后交换文件，由用户自行选择是否运行外部工具。

## 已实现且具有辨识度的能力

以下十项均由发布标签、实际源码或官方文档支持；功能请求不混入此表。

| 已实现能力 | 用户价值与实现边界 | 一手证据 |
| --- | --- | --- |
| 文本会话、响应捕获和请求串联 | 创建资源后以 JSONPath/XPath 抽取 id、令牌供后续请求使用；有 Cookie 会话及 Cookie 文件。跨文件 `INCLUDE` 仍是另一项提案。 | [8.1.0 请求串联示例][4]、[捕获文档][13]、[执行模块][7] |
| 内容及元数据声明式断言 | 状态、版本、头、Cookie 属性、JSONPath、XPath、正则、证书、哈希与耗时；不必为每种常见检查编写脚本。完整正文比较仍是精确比较。 | [断言文档][12] |
| 类型化变量、动态值与秘密输出遮蔽 | JSON 模板可插入字符串、数值、布尔及空值；支持变量文件、环境输入、`newUuid` 等函数。`--secret` / `--secrets-file` 对指定值遮蔽日志/报告，不等于凭据保险库。 | [固定版示例和手册][4]、[模板文档][14] |
| 与断言联动的重试、延迟和重复 | 创建异步任务后，按 `retry-interval` 重试直到状态断言通过；另有 `delay`、`repeat`、`skip`。作者须自行处理重复执行副作用。 | [固定版 Polling and Retry][4]、[CLI 手册][11] |
| 文件级测试执行器 | `--test` 支持多文件、递归目录、`--glob`；默认并行文件，`--jobs 1` 可串行。文件内部请求仍顺序运行。 | [运行测试文档][15] |
| CI 报告与失败退出码 | HTML、JSON 目录、JUnit、TAP；退出码为成功 0、选项错误 1、文件解析错误 2、运行错误 3、断言错误 4。`--report-json` 中的响应体是落盘文件引用。 | [固定版 Reports / Exit Codes][4]、[报告文档][15] |
| libcurl 认证及传输控制 | Basic、Digest、NTLM、Negotiate、AWS SigV4、客户端证书、TLS 上下限；HTTP/2、HTTP/3 能力按实际构建探测。Bearer/API Key 可通过头或查询表达。 | [实际客户端源码][8]、[CLI 手册][11] |
| 格式检查及 curl 互通 | `hurlfmt` 有检查、原地格式化、Curl/Hurl 输入、Hurl/Json/Html 输出；Hurl 有 `--curl` 调试输出。格式化器 JSON 是语法表示，不是 Postman 集合格式。 | [选项枚举及解析][9]、[执行分支][10]、[固定版手册][4] |
| 分层默认配置 | 8.1.0 交付配置文件；实际路径为 `$XDG_CONFIG_HOME/hurl/config` 或 `$HOME/.config/hurl/config`，覆盖优先级由低到高为配置文件、环境变量、命令行、请求 `[Options]`。 | [发布清单][3]、[固定版 Configuration][4] |
| 轻量性能回归 | `duration` 毫秒门槛，`--json` 提供 DNS、连接、TLS、首字节和总耗时，配合 `--repeat`、`--jobs` 重复并行。未验证分布式施压、到达率或分位数报告。 | [性能测试文档][15]、[固定版示例][4] |

特别注意：`--json` 标准输出、`hurlfmt` 的 JSON 语法表示、`--report-json` 报告目录是三种不同产物。不能因为其中一种包含响应文件引用，就宣传另一种已默认内联完整响应。

## 与当前 Reva 的源码对照

下表中的缺项结论同时检查命令入口、模型、服务、GUI 和脚本，不以 README 未提及为依据。本地证据链接固定在当前工作树；复核时应使用上述基线提交。

| 领域 | Hurl 的当前能力 | Reva 当前实现与差异 | 对 Reva 的启示 |
| --- | --- | --- | --- |
| 定位与架构 | Rust/libcurl 文本会话测试，CLI 及 Rust 库。 | [main.go][28] 启动 Viva 画布；[app.go][29] 的 `Send` 以单个标签页创建发送快照和 Effect；Go 服务与 GUI 分层。 | 两者互补；保留本地探索体验。 |
| 协议 | HTTP/1.x、HTTP/2、构建允许时 HTTP/3；SOAP/GraphQL 是 HTTP 正文用途。 | [HTTP 服务][30] 基于 `net/http`，默认标准传输可协商 HTTPS HTTP/2；[GraphQL 编辑器][31]、[WebSocket 客户端][32] 已实现。未发现 HTTP/3/gRPC 发送入口。 | WebSocket 是 Reva 现有能力；HTTP/3 专项验证可交给外部工具。 |
| 认证与 TLS | 还有 Digest、NTLM、Negotiate、AWS SigV4、客户端证书及 TLS 版本边界，部分依赖构建。 | [模型][33] 与 [认证解析][34] 有 Bearer/Basic/API Key、集合/文件夹继承及 None 屏障；[HTTP 服务][30] 剥离跨源认证并支持超时、TLS 校验、重定向。S3 同步签名不等于通用请求 SigV4。 | 先明确已有作用域；企业认证单独按需立项。双方均不能据当前证据承诺完整 OAuth2 生命周期。 |
| 变量、响应捕获及 Cookie | 捕获结果用于后续请求；同文件 Cookie 会话，可读写 Cookie 文件。 | [变量][35] 已做集合/环境覆盖和 `{{name}}` 替换；[脚本运行器][36] 的变量复制，`RunTests` 不返回修改值，[响应回调][37] 仅收结果和日志；HTTP 客户端无 `CookieJar`，响应 Cookie 展示不等于自动会话。 | 缺的是跨发送的显式临时变量/会话语义，并非完全没有变量。 |
| 脚本与断言 | 声明式查询和谓词；所核查接口未显示通用前后置 JavaScript 运行时。 | [script.go / api.js][36] 已有 Otto 超时、前置/响应脚本、`pm.test`、`pm.expect`、JSON 解析、深比较和耗时断言；异步回调被拒绝。 | 利用现有运行时补低门槛操作，避免另造 DSL，也不宣称全面兼容 Postman。 |
| 执行器与 CI | 文件并行、筛选、重试、重复、报告、断言失败退出码。 | [入口][28] 只有 GUI；[集合菜单][38] 没有运行集合动作；[服务接口][39] 和 [发送管线][29] 未形成批量测试执行器/无窗口报告。各标签页能各自发送，不等于集合编排。[仓库 CI][40] 是测试 Reva 本身。 | 专业执行及 JUnit 先交给 Hurl，避免把应用构建 CI 混同用户 API 测试 CI。 |
| 导入导出 | `.hurl` 输入、受限 curl 互通；未核实 Postman/OpenAPI/HAR 通用导入。 | [importer][41] 已有原生、Postman、HAR、curl、Swagger/OpenAPI JSON 审阅导入；[exportCollection][38] 导出原生 JSON；[requestCURLChecked][42] 复制 curl。没有 Hurl 适配器；多种导入不代表均可双向导出。 | 增加诚实、受限的文本测试交换，比重复开发复杂导入器更有价值。 |
| Mock / Schema | 已核查 CLI、查询文档及执行源码，未确认内置 Mock 或 OpenAPI/JSON Schema 契约验证；JSONPath 不是 Schema 校验。 | [OpenAPI 转换][41] 的 `sample` 明确不验证约束；[脚本 API][36] 没有 Schema 校验器；生产入口/服务无 Mock 服务。测试中的 fake/httptest 不算产品能力。 | 不把双方都未确认的功能写成竞品优势；完整契约验证和 Mock 按需接专用工具。 |
| 性能 | 耗时门槛、网络指标、重复与文件级并行，可做轻量负载测试。 | [trace.go / Response][30] [33] 已记录阶段时间、耗时、大小、截断；脚本可断言 `responseTime`。没有持续施压和性能汇总报告。 | 单请求诊断已有基础；大规模施压交给专业工具。 |
| 协作与本地数据 | 文本及本地报告可随 Git 审阅，运行不需要 Hurl 账户；未确认托管团队工作区或收费协作版。 | [storage][43] 为本地 JSON / `REVA_HOME`；[syncing][44] 有密码加密、快照与合并，[sync.go][45] 支持 WebDAV、S3、GitHub/Gitee。外部同步凭据可选，核心本地使用免账户。 | Reva 并非只有本地文件、完全没有同步；现有同步亦不等于实时多人编辑、权限审批系统。 |

## 六个实际功能议题及交付核验

只调用一次最近更新的 100 条 issue/PR 混合列表，排除 PR 后选择功能需求，再读六个议题及两个关联 PR 页面。样本为 **4 条 open、2 条 closed**；不是全量需求统计。以下标题、状态、标签和更新时间均来自抓取时 GitHub 元数据，时间为 UTC。[25]

| 议题与精确标题 | 状态、标签、更新时间 | 需求及交付判断 |
| --- | --- | --- |
| [#115 — Feature Requests: Imports][17] | **open**；`enhancement`, `topic: import`；`2026-10-20T10:01:43Z` | 复用登录等片段、传参、重命名捕获并保留原文件失败定位。“Imports”指测试包含，不是集合迁移。2026-10-20 有外部分支原型，作者明确不期待直接合并；页面无关联开发 PR。**开放提案，主线交付未核实**，不能列为已有 INCLUDE。 |
| [#1907 — include response bodies in the json output][18] | **open**；`enhancement`, `topic: reports`；`2026-11-05T10:57:59Z` | 用户要机器读取整段会话的响应，避免解析 stderr。维护者 2026-01-16 解释为兼容性及体积保留 `--json`，采用 `--report-json`；[当前文档][15] 已说明响应另存文件及引用。**原提案开放、替代路径已实现**；未验证 `--json-full` 或默认内联正文。 |
| [#3219 — Support curl ;type magic string in multipart/form-data request][19] | **open**；`enhancement`, `topic: multipart`；`2026-11-05T10:58:06Z` | 同一请求上传文件和具有 `application/json` 类型的文本元数据。维护者认可扩展需求，当前列入 9.0.0、无关联 PR。文件分段类型已有，[文档][16] 的文件替代方案不能证明文本分段 `;type` 已支持。**开放提案，未核实交付**。Reva 的 `FormField` 也没有逐段 Content-Type 字段。 |
| [#5397 — implicit body placeholders][20] | **open**；`enhancement`；`2026-10-30T19:16:38Z` | 在正文比较中忽略时间戳等字段。维护者说明现有隐式比较含空白，计划加入语义 JSON 差异和省略字段；评论晚于 8.1.0 发布。**开放提案**，不能把问号占位符、部分匹配或忽略顺序当成已交付语法。 |
| [#3629 — Add default config file feature][21] | **closed**；`enhancement`；`2026-10-24T17:33:48Z` | 复用默认选项并定义优先级。**关闭且核实交付**：[8.1.0 发布][3] 明列并关联 #3629，[固定版 README][4] 给出真实路径及覆盖顺序。关联的 [PR #4927][23] 已合并但主要更新规范，单凭它或议题关闭仍不足；初始提案的 `~/.hurlrc` 不是最终路径。 |
| [#5335 — Add --tlsv1.0 --tlsv1.1 --tlsv1.2 --tlsv1.3][22] | **closed**；`enhancement`, `open-to-contribution`, `topic: curl options`；`2026-10-22T18:21:01Z` | 限制最低 TLS 版本以验证安全策略及兼容性。**关闭且核实交付**：[PR #5352][24] 于 2026-10-22 合并，[8.1.0][3] 明列，[客户端源码][8] 实际调用 `ssl_min_max_version`。最终选项为 `--tls-min`，不能按旧标题声称四个别名存在。 |

本样本未选取“关闭但未交付/重复”的条目；这不表示上游不存在此类议题。尤其 #1907 展示了开放状态与部分替代方案已交付可以同时成立，#115 的可运行分支也不等于主线发布。

## 三个适合 Reva 的最小改进

### 1. P1：显式、仅内存的响应捕获变量

**场景与缺口：**登录得到 token、创建资源得到 id，下一次手工发送需要使用。Reva 当前 `RunTests` 不回传变量、脚本写入不跨发送保留，持久环境也不适合临时凭据；Hurl 的捕获串联说明这里有具体价值。

**最小 PR：**提供响应 JSON 的 JSON Pointer 提取动作，用户命名后放入可查看、清空的临时会话变量。首版仅标量，使用 `json.Number` 避免大整数先变浮点；临时作用域覆盖集合/环境、仅内存保存，失败不覆盖旧值。与前置脚本副本的优先级和会话关闭时机必须明确。自动 CookieJar、脚本网络权限、持久秘密存储不并入首版。

**依赖：**已有响应正文、`vars.Lookup/Apply`、`VarsFor` 及发送快照；需要新的临时作用域状态和明确生命周期，不需要第三方依赖。不得顺手改变现有 `pm.environment.set` 的隔离语义。

**有意义的验收：**使用 `httptest` 返回 token 和大整数 id，显式提取后下一请求收到正确值；缺失路径、非标量、截断正文、坏 JSON 均保留旧值且提示。取消发送不得污染会话；重建 App 后临时值消失；集合/环境磁盘 JSON 和历史均不出现替换后的秘密值。

### 2. P1：受限 Hurl 导出，复用专业 CI 执行器

**场景与缺口：**在桌面调通 HTTP 请求后，希望把文本提交 Git，补充期待结果与断言，再由现有 CI 运行并收集 JUnit。Reva 已有原生导出，但没有 Hurl 交换、无窗口批量执行和报告。这里优先集成专用工具，不在本轮重写执行器。

**最小 PR：**集合/所选请求导出 `.hurl`，先预览后写文件；仅承诺方法、URL、启用参数/头、JSON/文本正文以及既有认证继承解析能表达的部分。保留占位符，不自动展开环境秘密；不预设成功一定为 `HTTP 200`。脚本、上传、WebSocket、不可转换选项、变量名或重复项语义必须明确阻止并定位请求。用户自行安装并运行 Hurl，应用不自动启动命令、不捆绑原生库。

**依赖：**现有集合遍历、`auth.Resolve`、`vars`、导出对话框；须定义转义、变量命名及格式转换边界。此建议不依赖第一项，也不声称 JavaScript 断言能自动翻译为 Hurl。

**有意义的验收：**离线金样覆盖中文、引号、换行、重复查询、空头值、继承 None 屏障；输出顺序稳定、未配置断言不凭空增加状态预期、环境 token 不进入导出。包含脚本或不兼容选项时不写文件，报告精确位置。在已有 Hurl 的可选集成环境，对 `httptest` 运行导出文件并核对实际请求；仓库测试不下载 Hurl，不访问互联网。

### 3. P2：从响应生成语义 JSON 断言片段

**场景与缺口：**只核对业务字段、忽略时间戳和对象字段顺序。Reva 已有深比较，只缺从实际响应选字段生成断言的便捷路径；无需等待或照搬 Hurl #5397 的新语法。

**最小 PR：**用户选定 JSON Pointer，预览后将 `pm.test` / `pm.expect(...).to.eql(...)` 插入现有 `TestScript`，默认只比较选定字段。正确转义键名和字符串；拒绝会被 JavaScript 数字精度改变的预期值。沿用既有模型与脚本语言，不引入新断言 DSL。

**依赖：**现有 `api.js` 深比较和脚本编辑器/片段菜单；可复用第一项的 JSON Pointer 读取器，也能独立实现，不依赖 Hurl 导出。

**有意义的验收：**实际执行生成的脚本：对象键重排、未选择的时间戳变化仍通过；选定值或类型变化必须失败并有断言名称。覆盖斜线/波浪号/引号键、数组、null、中文；超安全整数、坏 JSON、截断响应应拒绝；取消预览不得修改原脚本。

P1 表示高价值的后续候选，不是本次交付承诺；没有因工具研究强行安排 P0。完整 Schema 验证、Mock、分布式压测仍应按具体需求评估专用工具，Hurl 的 JSONPath/重复运行不能自动替代这些能力。

## 验证、交付及局限

本次无新增行为测试、源码、依赖或图标。`TASK.md` 和本地 `.claude/skills/building-with-viva/SKILL.md` 确认缺失，按任务说明未进行 GUI 修改；已读 `AGENTS.md`、`README.md`、`docs/onboarding-prs.md` 及上述实际实现。检查仅为文档变更执行仓库要求，没有安装 Hurl 或仓库新依赖。

实际检查环境：`go version go1.27.3 windows/amd64`，进程内设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`，并以 `GOPROXY=off`、`GOSUMDB=off` 运行，避免检查时下载依赖。**不是仓库说明的 Go 1.26 或 macOS 验证**。四项命令全部退出 0，尾部如下：

```text
$ gofmt -l .
(空输出，退出 0)
$ go vet ./...
(空输出，退出 0)
$ go tool viva vet ./...
(空输出，退出 0)
$ go test ./... -count=1
?    github.com/runbrick/reva/internal/service/fake     [no test files]
ok   github.com/runbrick/reva/internal/service/request  1.532s
ok   github.com/runbrick/reva/internal/service/store    1.090s
ok   github.com/runbrick/reva/internal/storage          2.032s
ok   github.com/runbrick/reva/internal/syncing          3.115s
ok   github.com/runbrick/reva/internal/update           2.084s
ok   github.com/runbrick/reva/internal/vars             2.645s
ok   github.com/runbrick/reva/internal/websocket        1.235s
(退出 0)
```

JSON 解析、结构字段、UTF-8、六条议题元数据与原始 API 逐项比对，以及 Markdown 的 45 个来源定义、本地文件链接和代码围栏检查均通过。两份文档的新增不改变上面 Go 检查所针对的程序；最终差异空白检查另由 `git diff --cached --check` 确认。

局限：

1. 未安装或实跑 Hurl，HTTP/3、认证后端、性能与平台差异基于公开源码/文档，未宣称做过协议互通或性能基准。特别是 libcurl 的构建能力不能保证每个分发包一致。
2. 在线文档和议题可能在抓取后改变；主要实现判断尽量固定到 8.1.0，当前文档用于补充。只有一页最近更新列表，不代表全体需求及其相对频率。
3. “未确认”指本次限定检查中没有证据；不是证明整个生态绝对不存在相应扩展。商业边界只覆盖核查的仓库和官方资料。
4. 猜测的官方 `ci-integration.html`、`integration.html` 及若干 hurlfmt 文档路径返回 404，未作引用证据；相关判断改用可访问的测试文档和固定版源码。
5. 本地检查虽全通过，但 Go 版本与说明不同；未重跑 Go 1.26、macOS、真实桌面窗口，也未新增文档以外的行为验收。

## 编号证据来源

上游来源按 JSON `sources` 顺序编号；源码采用发布标签，议题保留精确英文标题。Reva 的本地源码作为后续编号，实际检查基线见开头。

1. [规范仓库][1]。
2. [8.1.0 LICENSE][2]。
3. [8.1.0 正式发布及变更清单][3]。
4. [8.1.0 README：功能示例、手册、安装依赖][4]。
5. [工作区 Cargo.toml][5]。
6. [hurl_core 模块][6]。
7. [runner 模块及库入口][7]。
8. [HTTP 客户端实际实现][8]。
9. [hurlfmt 选项和输入输出枚举][9]。
10. [hurlfmt 执行分支][10]。
11. [官方 CLI 手册][11]。
12. [响应断言][12]。
13. [响应捕获][13]。
14. [模板与变量][14]。
15. [运行测试、报告及性能][15]。
16. [请求格式][16]。
17. [#115：Feature Requests: Imports][17]。
18. [#1907：include response bodies in the json output][18]。
19. [#3219：Support curl ;type magic string in multipart/form-data request][19]。
20. [#5397：implicit body placeholders][20]。
21. [#3629：Add default config file feature][21]。
22. [#5335：Add --tlsv1.0 --tlsv1.1 --tlsv1.2 --tlsv1.3][22]。
23. [PR #4927：Add Hurl config file][23]。
24. [PR #5352：Add --tls-min option to set the minimum allowed TLS version][24]。
25. [一次 issue/PR 列表请求][25]，状态、标签和更新时间来源。
26. [latest release API][26]，发布身份与时间来源。
27. [仓库 API][27]，规范名称、默认分支和许可证元数据。
28. [Reva 命令入口][28]。
29. [Reva GUI 发送快照、Effect 与历史][29]。
30. [Reva HTTP 服务][30]，同目录 `options.go`、`trace.go`、`body.go`。
31. [Reva GraphQL 编辑器][31]。
32. [Reva WebSocket 客户端][32]，另见 `internal/gui/websocket.go`。
33. [Reva 模型][33]，认证、请求、响应及选项。
34. [Reva 认证继承、校验及应用][34]。
35. [Reva 变量替换][35]，另见 `internal/gui/env.go:VarsFor`。
36. [Reva 脚本运行器][36]，同目录 `api.js` 和 `script_test.go`。
37. [Reva 前置/响应脚本整合][37]。
38. [Reva 集合菜单和原生导出][38]。
39. [Reva 服务接口][39]。
40. [Reva 自身 CI][40]。
41. [Reva 审阅导入入口][41]，同目录 `openapi.go:sample`。
42. [Reva curl 复制][42]。
43. [Reva 本地数据目录与原子 JSON 写入][43]。
44. [Reva 加密快照、共享与合并][44]。
45. [Reva 可选同步后端入口][45]。

[1]: https://github.com/Orange-OpenSource/hurl
[2]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/LICENSE
[3]: https://github.com/Orange-OpenSource/hurl/releases/tag/8.1.0
[4]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/README.md
[5]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/Cargo.toml
[6]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/packages/hurl_core/src/lib.rs
[7]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/packages/hurl/src/runner/mod.rs
[8]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/packages/hurl/src/http/client.rs
[9]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/packages/hurlfmt/src/cli/options/mod.rs
[10]: https://github.com/Orange-OpenSource/hurl/blob/8.1.0/packages/hurlfmt/src/main.rs
[11]: https://hurl.dev/docs/manual.html
[12]: https://hurl.dev/docs/asserting-response.html
[13]: https://hurl.dev/docs/capturing-response.html
[14]: https://hurl.dev/docs/templates.html
[15]: https://hurl.dev/docs/running-tests.html
[16]: https://hurl.dev/docs/request.html
[17]: https://github.com/Orange-OpenSource/hurl/issues/115
[18]: https://github.com/Orange-OpenSource/hurl/issues/1907
[19]: https://github.com/Orange-OpenSource/hurl/issues/3219
[20]: https://github.com/Orange-OpenSource/hurl/issues/5397
[21]: https://github.com/Orange-OpenSource/hurl/issues/3629
[22]: https://github.com/Orange-OpenSource/hurl/issues/5335
[23]: https://github.com/Orange-OpenSource/hurl/pull/4927
[24]: https://github.com/Orange-OpenSource/hurl/pull/5352
[25]: https://api.github.com/repos/Orange-OpenSource/hurl/issues?state=all&per_page=100&sort=updated&direction=desc
[26]: https://api.github.com/repos/Orange-OpenSource/hurl/releases/latest
[27]: https://api.github.com/repos/Orange-OpenSource/hurl
[28]: ../../../cmd/reva/main.go
[29]: ../../../internal/gui/app.go
[30]: ../../../internal/service/request/request.go
[31]: ../../../internal/gui/graphql.go
[32]: ../../../internal/websocket/client.go
[33]: ../../../internal/model/model.go
[34]: ../../../internal/auth/auth.go
[35]: ../../../internal/vars/vars.go
[36]: ../../../internal/script/script.go
[37]: ../../../internal/gui/scripts.go
[38]: ../../../internal/gui/collections.go
[39]: ../../../internal/service/service.go
[40]: ../../../.github/workflows/ci.yml
[41]: ../../../internal/importer/result.go
[42]: ../../../internal/gui/request.go
[43]: ../../../internal/storage/storage.go
[44]: ../../../internal/syncing/sync.go
[45]: ../../../internal/gui/sync.go
