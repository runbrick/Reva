# Schemathesis：契约生成测试与 Reva 的分工

> 本地文档 PR；分支 `runbrick/research-schemathesis`，只增加本页与同名 JSON，不修改程序、不新增依赖或行为测试、不推送或合并。

- Reva 基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`（开始时 `git rev-parse HEAD` 实测）。
- 上游规范仓库：<https://github.com/schemathesis/schemathesis>；分类：`schema-test`。[1]
- 研究版本：`v4.33.4`，最新发布于 `2026-11-12T16:27:07Z`；不是根据旧版本印象判断。[3][4]
- 实际检索结束时间：`2026-11-16T22:38:10.316001+08:00`，本机时钟 `+08:00`；系统观察日期为 2026-11-16。
- 已读 AGENTS.md、README.md、docs/onboarding-prs.md；TASK.md 与指定本地 Viva skill 缺失，任务已明确无 GUI 改动。

## 结论与许可证边界

Schemathesis 是本地可运行的契约驱动 API 测试工具，长于生成边界/无效输入、自适应复用真实数据、状态链、失败缩减和 CI 工件；适合补足 Reva 的批量探索能力。Reva 已有脚本、GraphQL、WebSocket 与可选加密同步，应继续承担免账号桌面调试，而非重建同一模糊测试引擎。

**核心是 MIT 开源，不是受限源码许可。** 已核对发布标签 LICENSE 全文、pyproject 的 SPDX `MIT` 与 GitHub 仓库身份；MIT 属 OSI 认可许可。商业使用允许，但分发须保留版权和许可声明；这不是将所有第三方依赖统一许可为 MIT。[1][2][3]

**没有把旧云服务算进当前能力。** v4 迁移说明明确旧 Schemathesis.io 服务已停用，`auth`/`upload` 及云 token 选项被移除；本地 CLI、HTML/JUnit、baseline 不要求该托管账号。当前官网链接的 Workbench/TraceCov 是独立入口；尤其关键词覆盖率报告需额外 TraceCov 集成，不能由 Schemathesis 的 MIT 推定外部组件、托管服务或商业套餐的条款。所读当前页面未核实有效的付费团队套餐，不能据此宣称当前有商业 RBAC 或实时协作。[13][24][25][26]

## 定位与架构

Python 3.11+；pyproject 声明 schemathesis/st CLI 和 pytest 插件入口。schema 转换成 Hypothesis 策略；Engine 创建探测、schema 分析、认证准备和四个测试阶段的执行计划，以事件流驱动报告；HTTP 与可选 gRPC 传输负责真实调用，checks 判断响应，资源池/反馈改变后续生成，crash/cache/baseline 保存在本地。pytest 参数化并不包含 CLI 的全部反馈与基线能力。[3][6][7][17][19]

主要数据流是「schema → 操作/依赖 → 生成策略 → 真实调用 → 检查 → 反馈及工件」。完整 CLI 还包含探测与认证准备，不能把“四个测试阶段”误读成全部执行阶段。它对输入空间做搜索；Reva 当前的 `App.Send` 则围绕用户编辑的一条请求调用 Go service，并在响应后运行现有 JavaScript 断言。两者对应不同工作环节。

## 十项已实现能力

以下均有 `v4.33.4` 的发布文档/源码证据；开放提案单列在后，不混入能力清单。

| 能力 | 当前实现及限制 | 一手证据 |
| --- | --- | --- |
| 从多种契约生成真实请求 | OpenAPI/Swagger 2.0、3.0、3.1、3.2、GraphQL，以及 4.33.0 新增的 unary gRPC；gRPC 通过反射或描述符集加载，需要 grpc 可选依赖，不直接解析 .proto，不测试流式方法，也没有负向生成。 | [18] |
| 四阶段生成与失败缩减 | 示例、约束边界覆盖、随机模糊、状态序列分别探索输入；基于 Hypothesis 缩减失败用例，OpenAPI/GraphQL 可生成违反约束的数据。覆盖阶段不等于完整穷举或关键词覆盖率报告；gRPC 不运行示例阶段。 | [6] |
| 响应驱动的自适应测试 | 从服务端拒绝信息学习缺失的约束与认证要求，从成功响应收集真实 ID，后续操作混用真实值与随机值；部分发现缓存后会在下次运行重新探测，不是直接相信历史值。学习会收窄正向探索范围，不能保证发现所有服务端约束。 | [8] |
| 自动推导多步骤状态链 | OpenAPI 模型、显式 Links、Location 和同源正文 URL 建立生产者/消费者关系；GraphQL 类型图、gRPC 方法/字段约定也能连接操作。用创建返回的 ID 读取或删除资源；不是普通集合的固定顺序回放。推导有启发式边界。 | [10] |
| 契约与对象级授权检查 | 包括服务器错误、状态码、Content-Type、必需响应头、响应 schema、无效输入接受与有效输入拒绝；对象级授权检查需配置至少两个身份和可归属的对象，自动以其他身份重复读取，不重复写入。没有报告不代表所有授权路径已覆盖。 | [11] |
| 声明式认证及逐操作配置 | TOML 提供 Basic、静态头、按 OpenAPI security scheme 的凭据、登录端点取 token；${VAR} 来自操作系统环境，缺失即配置错误。复杂刷新可用 Python auth 类；不是完整浏览器 OAuth2 授权管理器，也不是 Postman 环境编辑器。 | [12] |
| 失败工件、重放与独立测试导出 | 自动保存失败，st replay 可按 ID 或目录重放，多步骤按链接重新提取值；可输出只依赖 requests 的 pytest 文件。导出的断言记录当时的失败症状，通常在 bug 仍存在时通过，修复后失败；固定 ID 的导出链也可能失效，不能直接当作永久正确性回归测试。 | [14] |
| 可审阅的 CI 失败基线 | 将已接受失败保存为本地 JSON，后续只因新失败阻断，更新/删除需显式选项，可加到期日。随机运行未观察到不等于修复；某些签名可能合并同类新缺陷，响应时间失败不进入基线。 | [15] |
| 可离线审阅的多格式报告 | HTML 单页含失败、警告、响应与复现命令，另有 JUnit、HAR、VCR、NDJSON、JSON、Allure 等输出。pytest 与 gRPC 支持的格式有子集差异。关键词级 schema 覆盖率另依赖 TraceCov，不能归为核心无附加条件的能力；响应正文未保证脱敏。 | [16] |
| 可控流量与 Python 扩展 | workers 并行执行操作、固定速率、CLI 的 auto 429/Retry-After 等待重试，以及时间预算；自定义 Python checks/hooks/strategies 和 ASGI/WSGI 进程内调用可扩展测试。并发与慢响应阈值不是负载压测的吞吐量或延迟分位数模型。 | [13] |

注意：单次运行可真实创建、修改、删除数据，覆盖阶段也可能尝试未声明的方法；应在可重建的本地/测试环境由用户明确执行。这个行为影响交接包的产品设计，不意味着研究过程执行过目标 API 测试。[6][26]

## 与实际 Reva 基线逐项对照

源码路径均相对仓库根目录，函数名用于定位；这里的“未发现”限于该基线已检索的生产入口与模型。

| 维度 | Schemathesis | Reva 当前实现（源码依据） | 对 Reva 的含义 |
| --- | --- | --- | --- |
| 定位与架构 | Python 3.11+ 的契约驱动测试引擎，CLI、pytest 插件和库；Hypothesis 负责生成基础能力、缩减与示例数据库，事件流驱动报告。[3][6][7] | Go/Viva 本地手动 API 工作台；service 接口隔离 UI、HTTP 与持久化，ui.Effect 负责发送。<br>`cmd/reva/main.go:main`、`internal/service/service.go:Services`、`internal/gui/app.go:Send/declareSends` | 保留 Go/Viva 主程序，把探索性调试与批量契约测试衔接。 |
| 协议 | HTTP OpenAPI、GraphQL、unary gRPC；gRPC 需要可选依赖且不支持流式；没有据此证实交互式 WebSocket 客户端。[5][18][19] | HTTP(S)、GraphQL query/variables 专用编辑、ws/wss 文本及二进制会话；本次源码范围未见 gRPC 客户端。<br>`internal/service/request/request.go:Send/requestURL`、`internal/gui/graphql.go:graphQLBodySection`、`internal/websocket/client.go:Dial`、`internal/gui/websocket.go` | gRPC 模糊测试交给专用工具；不为报告新增协议运行时。 |
| 认证 | Basic、头/API key、逐 security scheme 凭据与声明式登录；Python 扩展刷新，WFC 多身份授权检查。[12][20] | Bearer/Basic/头或 query API key、集合/目录继承与 no-auth 阻断、显式头优先；跨源重定向剥离生成的敏感头。OAuth2 生命周期暂未实现，WebSocket 仍靠手动头。<br>`internal/model/model.go:Auth`、`internal/auth/auth.go`、`internal/gui/auth.go:effectiveAuth`、`internal/service/request/request.go:protectRedirects`、`internal/websocket/client.go:Dial` | 先做好复现与逐运行变量，不把静态 token 输入误称 OAuth2 流程。 |
| 变量与跨请求数据 | TOML ${VAR} 环境变量、参数覆盖、字典、真实响应资源池及状态链接；文件字典的 API 动态填充/父子关联仍是提案 #5751。[8][10][13] | 集合变量 < 活动环境；{{name}} 替换 URL/头/正文/认证，未定义名称保持原样。前置脚本变量仅影响本次发送；响应脚本变量未返回到后续请求。<br>`internal/vars/vars.go:Lookup/Apply`、`internal/gui/env.go:VarsFor`、`internal/script/script.go:RunPre/RunTests`、`internal/gui/scripts.go:afterResponse` | 新增显式的运行内变量上下文，避免误写用户持久环境。 |
| 脚本与断言 | Python 检查、生成钩子、策略与认证提供者，pytest 集成；CLI 的资源池、反馈、baseline 不会自动出现在 @schema.parametrize() 下。[17][21] | Otto 执行 JavaScript，pm.test/pm.expect、响应 JSON/头、请求头编辑与日志；默认 2 秒限制，不支持异步测试。不是完整 Postman 沙箱。<br>`internal/script/script.go:DefaultTimeout/RunPre/RunTests`、`internal/script/api.js`、`internal/gui/scripts.go` | 沿用现有脚本语义做确定性 runner，避免移植 Python 插件生态。 |
| Runner 与 CI | 自动多操作生成、状态链、退出码、JUnit/JSON 与已知失败基线，适合 CI。[15][23] | App.Send 每个 tab 发送一次并运行脚本；唯一 cmd/reva 入口启动 GUI。本次检索未发现集合执行 CLI/JUnit 输出；现有 CI 检查的是 Reva 程序自身。<br>`internal/gui/app.go:Send/run/declareSends`、`cmd/reva/main.go`、`internal/service/service.go:RequestService`、`internal/script/script.go`、`.github/workflows/ci.yml` | 明确区分应用自身测试和用户集合测试；最小 runner 先顺序一次、支持取消。 |
| 导入与导出 | 直接消费 schema URL/文件；导出测试运行的 HAR/VCR/报告及失败 pytest，不是 Postman 集合工作区迁移器。[13][14] | 审阅后导入原生/Postman、HAR 请求、受支持 curl、Swagger/OpenAPI JSON；原生集合导出、curl 生成与加密备份已实现。OpenAPI 响应与约束被丢弃，不支持 YAML/外部引用。<br>`internal/importer/result.go:ParseWithOptions`、`internal/importer/openapi.go:sample`、`internal/gui/importer.go:reviewImport/performImport`、`internal/gui/collections.go:exportCollection`、`internal/gui/request.go`、`internal/gui/sync.go` | Schemathesis 的受支持 curl 或 HTTP HAR 可回 Reva 手调；HAR 会丢弃响应/时序，输入上限与不支持的二进制内容仍须审阅。 |
| Schema 验证 | 运行时检查实际响应，GraphQL data 类型检查与 OpenAPI schema 验证；4xx 业务语义分类仍有开放提案 #5549。[5][11] | OpenAPI 只生成可编辑请求样例；sample 注释明确不验证约束，model.Request 不保留响应契约。现有 pm.expect 支持手写断言，不是 JSON Schema 验证器。<br>`internal/importer/openapi.go:openapi/sample`、`internal/model/model.go:Request`、`internal/script/api.js`、`internal/i18n/import.go:OpenAPIReview` | 先保存最小响应契约做状态码/媒体类型提示；完整生成及 schema 验证可外接。 |
| Mock | 被测对象是真实运行的服务；生成请求和 ASGI/WSGI 进程内测试不等于提供 mock server，本次资料未验证产品化 mock 服务。[6][22] | 生产源码未发现用户可启动的 mock server；service/fake 是 GUI 测试替身，httptest 是测试工具。<br>`internal/service/fake/fake.go`、`internal/gui/harness_test.go`、`cmd/reva/main.go` | 若需 mock，另外评估专用工具；不把测试替身统计为产品能力。 |
| 性能与流量控制 | 并发、速率/Retry-After、运行预算与响应时间检查；可测慢接口，但不是经本报告验证的专业负载测试系统。[11][13] | HTTP 连接池复用、逐请求超时/重定向/TLS 设置、响应大小上限、DNS/连接/TLS/等待/下载时间线；无集合吞吐量、分位数或自动 429 重试流程。<br>`internal/service/request/request.go:Service/Send`、`internal/service/request/options.go`、`internal/service/request/trace.go`、`internal/model/model.go:Response/RequestOptions` | 保留单请求可解释时间线；性能压测另用专用工具，runner 不自动重试有副作用请求。 |
| 协作与本地数据 | TOML、失败缓存、baseline 和离线报告可进入本地/Git/CI 产物工作流；旧托管上传服务已停用。报告正文可能含敏感值，未证实团队权限/实时共编服务。[13][16][24] | 免 Reva 账号，REVA_HOME/用户配置目录存 JSON；可选 AES-GCM 备份、WebDAV/S3/GitHub/Gitee 加密快照同步和合并。远端供应商可要求凭据；这不等于 RBAC/实时共编，也不等于本地 JSON 已加密。<br>`internal/storage/storage.go:Dir/WriteJSON`、`internal/syncing/sync.go:Seal/Merge/Shared`、`internal/gui/sync.go:makeRemote`、`internal/cloudsync/cloudsync.go` | 沿用用户控制的数据目录和可选远端；共享报告单独审阅敏感正文，不新增强制账号。 |

尤其不能漏算 Reva 的现有能力：`internal/gui/graphql.go` 确实提供 query/variables 编辑；`internal/script/script.go` 与 `api.js` 确实实现前置脚本和响应断言；`internal/gui/sync.go:makeRemote` 确实接入 WebDAV/S3/GitHub/Gitee。另一方面 `RunTests` 只返回结果/日志/错误，响应脚本对变量副本的修改不会自动传给下一次发送；这才是工作流衔接的具体缺口。

## 功能议题：需求、状态与交付证据

检索先取最近更新的 100 项混合列表，再为功能研究补一次标题 `[FEATURE]` 搜索（匹配 216，返回前 100）；没有反复分页消耗 API 配额。讨论/关闭事件/PR 细节用 GitHub 直接页面读取。选取 7 个真实功能议题：**2 open、5 closed**；后者含 4 个有独立交付证据及 1 个明确不交付的范围决策。时间为上游 `updated_at`，不是关闭日期。

### #5751 — [FEATURE] Dictionaries filled from an API operation, with parent and child drawn together

- 议题：<https://github.com/schemathesis/schemathesis/issues/5751>；状态：`open`；更新：`2026-11-12T08:50:25Z`。
- 原始标签：`Status: Needs Triage`。
- 用户需求：既有文件字典需要预先抓取 ID，父子 ID 独立抽样常全为 404；用户希望从 API 填充字典并保留同一父子的关联。
- 交付判断：**开放提案；未验证交付，不能把现有资源池等同于提案中的 from-operation 字典**。
- 核验路径：[27] [8]；链接详见编号来源。

### #5549 — [FEATURE] Classify 4xx responses by body, not only by status code

- 议题：<https://github.com/schemathesis/schemathesis/issues/5549>；状态：`open`；更新：`2026-11-05T11:12:17Z`。
- 原始标签：`Status: Needs Triage`。
- 用户需求：部分 API 用统一 400 加正文 type 区分业务冲突与校验失败；只看状态码会把正常业务拒绝误判为 schema 缺陷。
- 交付判断：**开放提案；未验证按正文配置的统一 4xx outcome 规则已交付**。
- 核验路径：[28] [11]；链接详见编号来源。

### #5417 — [FEATURE] Apply captured resource ids to array items and `$ref` nested objects in request bodies

- 议题：<https://github.com/schemathesis/schemathesis/issues/5417>；状态：`closed`；更新：`2026-10-18T12:20:07Z`。
- 原始标签：`Status: Needs Triage`。
- 用户需求：批量创建或 $ref 嵌套字段也需要真实外键，不能只让最外层对象使用之前捕获的 ID。
- 交付判断：**已验证交付：PR #5423 已合并，4.32.0 变更记录明确列出数组项与 $ref 对象填充**。
- 核验路径：[30] [31] [5] [9]；链接详见编号来源。

### #3620 — Feature proposal: declarative JWT auth with fetching new tokens

- 议题：<https://github.com/schemathesis/schemathesis/issues/3620>；状态：`closed`；更新：`2026-03-19T14:05:20Z`。
- 原始标签：`Priority: High`、`Specification: OpenAPI`。
- 用户需求：不写 Python 就能声明登录端点、请求凭据并提取 token，降低认证 API 测试的上手成本。
- 交付判断：**已验证交付：PR #3623 已合并，4.13.0 记录及 4.33.4 认证文档确认声明式动态认证；不是浏览器 OAuth2 全流程**。
- 核验路径：[33] [34] [5] [12]；链接详见编号来源。

### #1722 — [FEATURE] Automatically delay requests based on the `Retry-After` header value

- 议题：<https://github.com/schemathesis/schemathesis/issues/1722>；状态：`closed`；更新：`2026-07-08T20:15:02Z`。
- 原始标签：`Priority: Low`、`Difficulty: Intermediate`、`UX: Usability`、`Core: Transport`。
- 用户需求：API 返回 429 后应尊重 Retry-After，避免固定限流配置难以匹配服务端动态窗口。
- 交付判断：**已验证交付：PR #4289 已合并，4.23.0 明确发布 --rate-limit=auto；pytest 不自动等待/重试 429**。
- 核验路径：[36] [37] [5] [13]；链接详见编号来源。

### #2999 — [FEATURE] Output tests run to recreate manually without having to use seed.

- 议题：<https://github.com/schemathesis/schemathesis/issues/2999>；状态：`closed`；更新：`2026-10-18T12:20:01Z`。
- 原始标签：`Status: Needs more info`。
- 用户需求：无需依赖随机 seed 或手工解析终端输出，能把失败序列导出为人工可运行的文件。
- 交付判断：**已验证交付的具体范围：PR #5347 已合并，4.32.0 发布独立 pytest 失败文件；不是任意格式完整运行导出，也不是自动生成修复后应通过的回归断言**。
- 核验路径：[39] [31] [5] [14]；链接详见编号来源。

### #3511 — [FEATURE] Test only untested new, or changed APIs

- 议题：<https://github.com/schemathesis/schemathesis/issues/3511>；状态：`closed`；更新：`2026-02-15T17:35:27Z`。
- 原始标签：`Status: Needs Triage`。
- 用户需求：大型 API 希望只测试新增或变更的操作，以减少 CI 成本。
- 交付判断：**已关闭但该自动差异检测未交付：维护者在关闭评论明确认定超出项目范围；现有 include/exclude 可接受外部 schema diff 的结果，COMPLETED 标记不能证明交付**。
- 核验路径：[40] [13]；链接详见编号来源。

#3511 的 GitHub 关闭原因虽然标为 COMPLETED，维护者评论却明确建议外部 schema diff 生成 include 参数；因此这里归为“关闭但范围外未交付”。#2999 的独立 pytest 文件确已发布，但其断言复刻失败症状，不能把议题关闭解释成全部导出愿望或正确性回归自动完成。[14]

## 建议的三个最小 PR

均为后续建议，本次只交付研究文档；没有证据支持必须立即抢修的 P0，因此优先级为 P1/P1/P2。

### 1. P1：导入时保留最小响应契约并提示状态码/媒体类型漂移

- 用户场景：开发者导入 OpenAPI 后，服务从声明的 201 application/json 变成 200 text/html，希望在单次调试时立即看到偏差。
- 现有缺口：目前导入丢弃 responses，Request 没有契约关联；仅凭导入成功或 JSON 美化无法验证响应。
- 最小 PR：只为 OpenAPI JSON 导入保留有大小上限的每操作响应状态键和媒体类型清单、来源与操作标识；单次响应后显示独立的契约提示，支持明确关闭。先支持 exact/2XX/default 状态选择和媒体类型匹配，无法支持的引用显式标为未检查，不声称完整 JSON Schema 校验；按现有 schema-v1 严格导入约束明确兼容策略。
- 依赖/所有权：需协调 model、importer、store、原生导入导出、gui 与 zh/en 文案所有者；本报告不修改这些文件。只用 Go 标准库做有限检查，不新增 JSON Schema 依赖；完整契约验证先交给 Schemathesis。
- 有意义的验收：用本地 httptest 依次返回声明的 201 JSON、未声明的 200 HTML、2XX/default 覆盖与带 charset 的合法类型，验证提示及选择规则；保存/重启/原生导出再导入后保留相同契约；缺失契约和超出支持范围明确显示未检查，导入不发送请求。

### 2. P1：增加免 GUI 的确定性集合顺序执行入口

- 用户场景：本地和 CI 复用同一集合完成登录→创建→读取，用响应中的 ID 验证后续请求，并获得失败退出码。
- 现有缺口：已有脚本与可复用 RequestService，但发送入口属于单个 tab，响应脚本变量不返回给下次请求，也没有用户集合的 CLI/JUnit 结果。
- 最小 PR：新增独立 cmd/reva-run 与无 UI 执行服务，按选定集合/目录顺序执行一次，复用认证继承、选定环境和逐请求选项；增加返回变量的脚本 helper，将响应 pm.variables.set 仅提交到本次运行上下文，保持旧单次调用语义。首版只输出结构化 JSON 和 0/1/2 退出码，显式取消与失败即停策略，不含并行、循环数据集、自动重试或云服务。
- 依赖/所有权：需协调 service、auth、script、vars 与命令入口；共享现有实现，运行只读集合/环境，临时状态不写用户历史或环境。与契约提示 PR 可独立推进；JUnit 可后续单独 PR。
- 有意义的验收：临时数据目录加 httptest：创建返回新 ID，测试脚本放入运行变量，第二请求必须发送此 ID；断言失败退出 1、配置错误退出 2、超时/取消不再发送后续操作；运行前后磁盘 JSON 字节相同，报告不泄露 token，测试全程无需互联网。

### 3. P2：导出可审阅的 Schemathesis 测试交接包

- 用户场景：用户在 Reva 调通一个带认证的 API 后，希望探索 schema 边界、状态链与对象级授权，把发现的 HTTP 失败带回桌面定位。
- 现有缺口：Reva 不保留完整 schema，也没有生成/缩减/资源推理引擎；自行复制会引入 Python 运行时或大量新算法和维护负担。
- 最小 PR：由用户选择原始 OpenAPI JSON 或 URL，生成固定版本的 Schemathesis TOML 和 zh/en 说明、可复制命令，默认引用 ${API_TOKEN}，不展开 Reva 密钥，不自动安装或执行。说明选本地/测试环境、请求预算、关闭自动注册的选项、HTML/JUnit 产物与 baseline 复核；失败通过受支持 curl 或有损 HTTP HAR 导入 Reva，不能把当前原生集合伪装成完整 OpenAPI。
- 依赖/所有权：需要导出/导入 UI 和 i18n 协调；若要自动使用导入时原 schema，另需一个有界原文快照 PR，最小版可手动指定原文件而独立实施。Schemathesis 由用户另装；gRPC extra、TraceCov、压测和 mock 都不打包进 Go/Viva 主程序。
- 有意义的验收：用含中文路径、引号和 shell 元字符的本地 schema 地址与测试凭据生成交接包，解析 TOML 验证值完整、没有真实 secret，生成过程零进程执行/零联网；用离线 HTTP HAR/curl 样本导回 Reva，检查方法/URL/正文与兼容通知；用户明确执行前没有请求或数据变更。

生成/缩减、资源依赖推理、多身份授权与复杂 schema 验证优先外接 Schemathesis；schema 变更检测按 #3511 的建议交给专用 diff；压测和 mock 分别选专门工具。Reva 自己承担单次诊断、显式断言、可解释的确定性执行与交接，不需要把 Python/云账号引入主程序。

## 限制

- 能力结论以发布标签 v4.33.4 的文档和源码为主，初始 master README 只用于发现入口；latest 发布信息在检索时为 2026-11-12 的 4.33.4。标签不是不可变 commit 固定；议题状态与官网内容会变化。
- 检索窗口：先读一次最近更新的 100 项 issue/PR 混合列表，再因其多数为 PR，补一次标题含 [FEATURE] 的 issue 搜索（共匹配 216，返回前 100）；从中选 7 项，另用直接页面核查讨论、关闭事件与 4 个合并 PR。没有遍历全部 216 项或所有历史评论。
- 7 项功能议题含 2 open、5 closed；closed 中 4 项有独立交付证据、1 项明确超出范围。未用常规 bug 数量替代功能研究；不能从 closed/COMPLETED 自动推出已发布。
- 未安装或运行 Schemathesis、TraceCov、gRPC extra，也未做性能实测；报告中的并发、覆盖和授权行为是上游发布资料与源码的证据结论，不是本机端到端认证。
- MIT 结论只覆盖所核验的 Schemathesis 仓库代码/文档；TraceCov、托管 Workbench、Docker 基础组件、第三方库与供应商服务条款未做独立许可证审计。当前页面未核实有效的商业套餐，不能由旧服务历史或官网关联链接推断商业功能。
- 否定项限于已读的 Reva 基线与列出的官方资料/生产代码入口；没有 README 条目不是不存在的证明。Reva service/fake、httptest、自身 CI 与脚本 runner 私有类型均未冒充用户产品功能。
- 系统实际返回检索时间为 +08:00，日期 2026-11-16；未把提示中的 America/New_York 误写成系统时区。检索结束时间单独记录，不声称每个 HTTP 请求同时完成。
- 必需检查在 Windows/amd64、现有 Go 1.27.3 执行；go.mod 目标 1.26.0，未额外下载 1.26 工具链，也未验证 macOS 或原生窗口渲染。GOPROXY/GOSUMDB 关闭以避免下载依赖；未新增行为测试。

## 本地验证记录

文档专用改动未增加测试名、图标或源码修改，也没有需要其他文件所有者本次代改的请求。现有检查运行时进程环境设为 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`、`GOSUMDB=off`，使用已安装缓存；未安装仓库依赖，测试未访问互联网。

```text
$ go version
go version go1.27.3 windows/amd64
$ gofmt -l .
(empty)
Exit: 0
$ go vet ./...
(empty)
Exit: 0
$ go tool viva vet ./...
(empty)
Exit: 0
$ go test ./... -count=1
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	1.261s
ok  	github.com/runbrick/reva/internal/service/store	2.449s
ok  	github.com/runbrick/reva/internal/storage	1.120s
ok  	github.com/runbrick/reva/internal/syncing	1.977s
ok  	github.com/runbrick/reva/internal/update	0.499s
ok  	github.com/runbrick/reva/internal/vars	0.616s
ok  	github.com/runbrick/reva/internal/websocket	0.619s
Exit: 0
```

JSON 结构、UTF-8、议题元数据、Markdown/JSON 对应关系、文件边界与 `git diff --check` 在提交前另行校验。

## 编号证据来源

除官网、仓库身份、议题与发布/PR 页面外，核心功能链接固定到 `v4.33.4`。检索通过 GitHub API（身份/最新发布/两次列表）及直接页面、raw.githubusercontent.com 完成；以下均为实际取回内容或已核对变更记录对应的发布链接。

1. [规范仓库与项目身份](https://github.com/schemathesis/schemathesis)
2. [发布标签的 MIT 许可证全文](https://github.com/schemathesis/schemathesis/blob/v4.33.4/LICENSE)
3. [版本、许可证、运行时依赖与命令入口](https://github.com/schemathesis/schemathesis/blob/v4.33.4/pyproject.toml)
4. [最新发布：4.33.4，2026-11-12](https://github.com/schemathesis/schemathesis/releases/tag/v4.33.4)
5. [截至 4.33.4 的发布变更记录](https://github.com/schemathesis/schemathesis/blob/v4.33.4/CHANGELOG.md)
6. [数据生成、阶段、有效与无效输入](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/explanations/data-generation.md)
7. [执行计划与事件流实现](https://github.com/schemathesis/schemathesis/blob/v4.33.4/src/schemathesis/engine/core.py)
8. [响应反馈、资源池与跨运行缓存](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/explanations/adaptive-testing.md)
9. [资源池及关联值复用实现](https://github.com/schemathesis/schemathesis/blob/v4.33.4/src/schemathesis/specs/openapi/extra_data_source.py)
10. [操作依赖、自动推导与显式链接](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/explanations/stateful.md)
11. [内置检查与响应契约](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/reference/checks.md)
12. [静态认证、声明式登录与刷新扩展](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/auth.md)
13. [配置、变量、并发、限流和报告参考](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/reference/configuration.md)
14. [失败重放与独立 pytest 文件的限制](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/crash-reproduction.md)
15. [增量失败基线与局限](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/baseline.md)
16. [离线 HTML 报告与敏感响应正文](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/html-report.md)
17. [pytest 与完整 CLI 功能的边界](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/explanations/pytest.md)
18. [gRPC 支持与当前限制](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/grpc.md)
19. [gRPC 传输实现](https://github.com/schemathesis/schemathesis/blob/v4.33.4/src/schemathesis/transport/grpc.py)
20. [多身份对象级授权检查](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/object-level-authorization.md)
21. [Python 检查、钩子与策略扩展](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/extending.md)
22. [进程内 ASGI、WSGI 应用测试](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/python-apps.md)
23. [持续集成与报告产物](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/cicd.md)
24. [旧托管服务停用与 v4 迁移边界](https://github.com/schemathesis/schemathesis/blob/v4.33.4/MIGRATION.md)
25. [关键词覆盖率依赖独立 TraceCov 集成](https://github.com/schemathesis/schemathesis/blob/v4.33.4/docs/guides/coverage.md)
26. [当前官网的产品定位与功能入口](https://schemathesis.io/)
27. [[FEATURE] Dictionaries filled from an API operation, with parent and child drawn together](https://github.com/schemathesis/schemathesis/issues/5751)
28. [[FEATURE] Classify 4xx responses by body, not only by status code](https://github.com/schemathesis/schemathesis/issues/5549)
29. [[FEATURE] Apply captured resource ids to array items and `$ref` nested objects in request bodies](https://github.com/schemathesis/schemathesis/issues/5417)
30. [功能交付证据：5423](https://github.com/schemathesis/schemathesis/pull/5423)
31. [功能交付证据：v4.32.0](https://github.com/schemathesis/schemathesis/releases/tag/v4.32.0)
32. [Feature proposal: declarative JWT auth with fetching new tokens](https://github.com/schemathesis/schemathesis/issues/3620)
33. [功能交付证据：3623](https://github.com/schemathesis/schemathesis/pull/3623)
34. [功能交付证据：v4.13.0](https://github.com/schemathesis/schemathesis/releases/tag/v4.13.0)
35. [[FEATURE] Automatically delay requests based on the `Retry-After` header value](https://github.com/schemathesis/schemathesis/issues/1722)
36. [功能交付证据：4289](https://github.com/schemathesis/schemathesis/pull/4289)
37. [功能交付证据：v4.23.0](https://github.com/schemathesis/schemathesis/releases/tag/v4.23.0)
38. [[FEATURE] Output tests run to recreate manually without having to use seed.](https://github.com/schemathesis/schemathesis/issues/2999)
39. [功能交付证据：5347](https://github.com/schemathesis/schemathesis/pull/5347)
40. [[FEATURE] Test only untested new, or changed APIs](https://github.com/schemathesis/schemathesis/issues/3511)
