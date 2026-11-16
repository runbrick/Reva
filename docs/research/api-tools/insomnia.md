# Insomnia 对 Reva 的功能研究

- 检索时间：`2026-11-16T22:29:46+08:00`（实际执行环境时区偏移；本地观察日期 2026-11-16）。
- Reva 起始基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；分支：`runbrick/research-insomnia`。
- 规范仓库：https://github.com/Kong/insomnia；类别：`desktop-client`。
- 上游源码快照：`51e449474b832760d6cdd1e47d53b572d2b24ddf`（develop，提交时间 2026-11-16T09:29:38Z）；所见最新正式版：`core@13.4.0`，发布于 2026-10-29。[4]

Insomnia 的可借鉴优势在于把多协议调试、分层变量、数据驱动集合运行和规范/CI 工作流连在一起；Reva 已有脚本、GraphQL、WebSocket、共享认证和可选加密同步，应优先补可复现执行与变量透明度，保持本地免账户与 Go/Viva 方向。

## 定位、架构与许可边界

TypeScript/Node 单仓多包，桌面使用 Electron/React，依赖原生 HTTP 库及 gRPC 库；Inso CLI 共享 Insomnia 模块并单独暴露自动化命令。Local Vault、Scratch Pad、Git Sync、Cloud Sync 是不同存储/账户路径。Mock 服务为外部部署，不能把自托管等同于主客户端内置进程或开源授权。 [1] [3] [22]

**主仓库为 Apache-2.0，属于 OSI 开源；自托管 Mockbin 不是同一许可。** Kong/insomnia 主仓库与其中桌面/Inso 源码；Apache-2.0 属 OSI 开源许可。商业账户、托管服务与企业权益另计；关联 insomnia-mockbin 的 self-hosted 分支仅源码可见，LICENSE 指向 Kong Software License Agreement，不能套用主仓库许可证或称 OSI 开源。 [2] [16] [17]

不能把“源码开源”“客户端免费可用”“无需账户”“所有协作免费”混为一谈：

- **无账户本地模式**：Scratch Pad 可用，现行文档说 API Collection 可以包含请求及一份 OpenAPI 规范；并非只有单个临时请求。Local Vault 指项目数据不上传，不意味着全部功能无需登录。[1] [18]
- **社区与商业边界**：定价页含 Essentials、Pro、Enterprise；当前存储页明确 Essentials 可选择启用 Git Sync 并限制每组织三人，或关闭 Git Sync 后允许不限用户。README 把 Git Sync 概括为 premium，存在口径差异；这里以现行套餐专门说明为准，不写“Git Sync 一律付费”。SSO/组织治理等受套餐限制。[19] [18] [1]
- **mock 边界**：云端 mock 按套餐有配额；自托管需要单独的 Mockbin、Docker/Redis。其 self-hosted README 直称 “source visibile, but not open-source”，LICENSE 指向 Kong Software License Agreement，无标准 OSI SPDX 标识可沿用，不能将其归为 Apache-2.0。[14] [15] [16] [17]
- **秘密与遮罩**：Secret 在私有全局子环境由 vault key 本地加密，默认不向脚本开放，也不随私有环境导出/同步；Confidential 自 13.4 起可隐藏显示，但落盘、导出、Git 中仍是明文。两者都不能被泛化为“整个本地数据库已加密”。[7] [4]

## 已实现的九项特色能力

“已实现”依据现行官方文档或实际源码；账户/服务条件见上文，不表示各项都免费免登录。

| 能力 | 实现与可核对证据 |
| --- | --- |
| 多协议请求工作台 | HTTP、GraphQL、gRPC、WebSocket 与 SSE 有独立请求入口；并非只把协议名写进产品定位。此处不将同属 HTTP 的 SOAP 视为独立传输协议。 [5] |
| 多种认证配置 | 官方列明 Basic、Digest、OAuth 1.0/2.0、Microsoft NTLM、AWS IAM v4、Bearer、Hawk、Atlassian ASAP 和 Netrc；OAuth 生命周期比手填令牌完整。 [6] |
| 分层环境与本地秘密 | 文件夹、集合基础/子环境、全局基础/子环境依序覆盖；私有子环境不导出或同步。Secret 在私有全局子环境中由 vault key 加密；13.4 的 Confidential 仅界面隐藏，落盘和导出仍明文。 [7] |
| 可异步执行的前后置脚本 | 脚本可修改请求/环境、使用 insomnia.sendRequest 发额外请求，支持顶层 await 和内置 Chai、ajv、tv4 等库；不能据此宣称每个响应自动接受 OpenAPI 契约校验。 [8] |
| 数据驱动集合运行器 | GUI 可选择请求、调整顺序、设置迭代与延迟；CLI 的 --iteration-data 支持 CSV/JSON，--iteration-count 支持重复执行；议题 #3294 也有已交付说明。 [10] |
| 同一项目数据的 CI 命令行 | Inso 提供 run collection、run test、lint spec、export spec；--ci 禁用交互，--workingDir 可指定 .insomnia、导出文件或本地数据，集合运行支持 JSON 结果文件。传统 unit tests 自 13.3 起默认隐藏并计划弃用，新工作流优先脚本。 [11] |
| 规范编辑与规则校验 | 集合内编辑/预览 OpenAPI 并生成请求，GUI 与 CLI 可使用自定义 lint；源码直接调用 Spectral。规范 lint、导出文件 JSON Schema 校验和实时响应校验是三件不同的事。 [12] |
| 云端与自托管 mock | 可由请求响应建立 mock；云端请求量受套餐配额，自托管指向 Mockbin 的 Docker/Redis 服务。Mockbin self-hosted 分支声明仅源码可见并受 Kong 商业许可约束，不能称整条 mock 链路为 OSI 开源。 [14] |
| 多种数据归属与协作方式 | Scratch Pad 无账户且数据本地；另有 Local Vault、Git Sync 和 Cloud Sync。当前 Essentials 可在启用 Git Sync 时限三名用户，或关闭 Git Sync 后使用不限用户配置；企业 SSO/治理不能视为免费社区默认能力。 [18] |

## 与当前 Reva 的直接比较

下面每项 Reva 证据都指起始基线的实际文件/符号。未发现入口的结论来自代码检查；不是从 README 没提到推导。

| 领域 | Insomnia | Reva 当前实现与证据 | 可取方向 |
| --- | --- | --- | --- |
| 协议 | HTTP、GraphQL、WebSocket、gRPC、SSE；见请求文档。 | HTTP(S)、GraphQL query/variables 编辑和自有 WebSocket 客户端已实现；HTTP Send 聚合完整响应，未见专用 SSE 事件流视图、gRPC 模型或调用入口。<br>代码：`internal/service/request/request.go:Send；internal/gui/graphql.go:initGraphQLBody/graphQLBodySection；internal/gui/websocket.go；internal/websocket/client.go；internal/model/model.go:Request` | 保留现有 GraphQL/WS 成果；gRPC 优先借助专用客户端，SSE 单列后续需求。 |
| 认证 | 支持 OAuth 1.0/2.0、Digest、NTLM、AWS IAM v4 等。 | 请求与集合/文件夹继承已有 Basic、Bearer、header/query API key；AuthType 没有 OAuth2。S3 存储后端虽有 SigV4，不等于请求认证界面支持 AWS IAM。<br>代码：`internal/model/model.go:AuthType；internal/auth/auth.go:Resolve/Validate/ApplyHeaders；internal/s3/sigv4.go` | 按实际客户场景先做受限 OAuth2 client_credentials；避免把同步后端能力算作请求功能。 |
| 变量与数据保护 | 文件夹/集合/全局分层，私有环境、加密 Secret、明文 Confidential；13.4 发布说明还列出变量来源预览。 | 集合变量与当前环境合并，后者覆盖；脚本变量仅作用于一次发送。环境、认证落盘是普通 JSON；加密备份不等于本地变量 vault，输入 Secure 也不等于加密。<br>代码：`internal/gui/env.go:VarsFor；internal/vars/vars.go:Lookup/Apply；internal/script/script.go:RunPre；internal/storage/storage.go:WriteJSON；internal/syncing/sync.go:Seal` | 先展示来源和最终生效值；秘密存储另行设计，不能把掩码当作安全承诺。 |
| 脚本与链式流程 | 异步脚本、额外网络请求、环境写入、内置 Chai/ajv。 | Otto 隔离执行，默认两秒，提供 pm.request、pm.variables、pm.environment、pm.test/expect 和 console；断言是有限子集，拒绝异步测试，没有 require/pm.sendRequest；前后置运行使用副本，响应测试变量不会持久化到后续请求。<br>代码：`internal/script/script.go:RunPre/RunTests/newRunner；internal/script/api.js；internal/gui/scripts.go:prepareSend/afterResponse` | 已有脚本基础可复用；先明确兼容范围，不承诺完整 Postman/Node 运行时。 |
| 运行器 | GUI 顺序/迭代/延迟；CLI CSV/JSON 数据驱动及结果报告。 | App.Send 和 run 面向单标签单请求；集合菜单是新增/导入/导出等，检查 cmd 和 internal 后未找到集合批执行入口。脚本内部 runner 类型仅执行一个脚本。<br>代码：`internal/gui/app.go:Send/run/declareSends；internal/gui/collections.go；cmd/reva/main.go；internal/script/script.go:runner` | 复用发送服务与脚本做本地串行 CLI 第一版，随后再讨论数据文件与链式状态。 |
| 导入导出与迁移 | 导入 curl、Postman、HAR、OpenAPI 等；现行原生导出为 v5 YAML/HAR，兼容旧 v4 JSON；CLI 能导出 OpenAPI。 | 已有审核后确认的 Native/Postman 环境与集合、HAR、curl、Swagger2/OpenAPI3 JSON 导入，以及集合追加；原生集合 JSON 导出和独立加密备份。明确拒绝 Insomnia 原生格式与 YAML，Postman 脚本有遗漏告知。<br>代码：`internal/importer/result.go；internal/importer/diagnostics.go；internal/importer/postman.go；internal/importer/openapi.go；internal/gui/importer.go；internal/gui/collections.go:exportCollection` | 现阶段以 HAR、curl 或双方能处理的 OpenAPI JSON 迁移请求；不承诺保留环境/脚本/测试，也不假设 Insomnia 可导出 Postman。 |
| mock | 已有云端与自托管 mock，后者需独立服务且有商业许可边界。 | 响应可查看/保存，但生产源码未见 mock 路由模型、服务监听或生成 mock 的入口；测试 httptest 不是用户 mock 产品。<br>代码：`internal/model/model.go；internal/gui/response.go；cmd/reva/main.go；internal/service/service.go` | 优先导出/保存响应并配合独立 mock 工具；不内置 Redis/Docker 或复制源码可见服务。 |
| schema-validation | Spectral 检查 OpenAPI；脚本内置 ajv/tv4 可手写响应校验；v5 导出也有文件结构 JSON Schema。 | OpenAPI 导入负责有限本地引用/转换与诊断，不保存完整契约；GraphQL variables 的 JSON 校验是语法/对象检查，pm.expect 是有限断言，未发现内置通用 JSON Schema 验证器。<br>代码：`internal/importer/openapi.go；internal/gui/graphql.go:graphQLBodySection；internal/script/api.js；internal/model/model.go:Collection` | 规范 lint/契约生成测试交给 Spectral 或专门的 schema 工具；当前导入成功不能当作接口契约合格。 |
| CI | Inso 可直接使用版本化项目数据执行集合或 lint，并禁用提示。 | 项目 GitHub Actions 验证 Reva 自身构建/打包；唯一 cmd/reva 入口启动 GUI，没有给用户集合使用的 headless API 回归命令。<br>代码：`cmd/reva/main.go；cmd/reva/services.go；.github/workflows/ci.yml；.github/workflows/release.yml` | 新增独立命令，给出稳定失败退出码和机器可读结果；不要将软件自身 CI 混为用户测试功能。 |
| 性能 | 迭代、延迟、响应时间与运行报告方便功能回归，但现有证据不足以称为高并发压测平台。 | 已有 DNS/TCP/TLS/等待/下载时间线、超时/取消、连接池复用、默认最多保留十 MiB 响应体；没有面向负载的并发、到达率和百分位 SLO 测试入口。未实测双方启动、内存或吞吐。<br>代码：`internal/service/request/trace.go；internal/service/request/request.go:Service/Send；internal/model/model.go:Response/Phase` | 负载与容量测试优先交给 k6 等专用工具；保留 Reva 单次排障时间线。 |
| 协作 | Git Sync 有提交/分支/冲突工作流，Cloud Sync 有协作与组织治理；具体人数和 SSO 受套餐约束。 | 无需 Reva 账户，已有加密快照同步，可选 WebDAV、S3、GitHub/Gitee Gist，有冲突检测/合并；没有组织 RBAC/实时共同编辑。Gist 不等同 Git 仓库分支协作。<br>代码：`internal/gui/sync.go:syncSettings；internal/syncing/sync.go；internal/cloudsync/cloudsync.go:Store；internal/gist/gist.go` | 继续使用用户自己的存储和 Git 工具；不引入 Reva 强制账户或自建组织服务。 |
| 本地数据与架构 | Electron/React/TypeScript 单仓多包；HTTP 原生库与 gRPC 库，桌面与 Inso 共享模块。Scratch Pad 无账户；Local Vault 本地存储不意味着所有功能免登录。 | Go/Viva 自绘桌面、无 Electron/WebView；REVA_HOME 可选择本地目录，JSON 原子写入，可由用户自行放入 Git；同步是可选项。<br>代码：`README.md；go.mod；cmd/reva/main.go；internal/storage/storage.go:Dir/WriteJSON；internal/gui/sync.go` | 把无需账户、可读数据和单一 Go 二进制作为设计约束；不根据技术栈断言更快或更省内存。 |

比较涉及的官方证据：协议 [5]；认证 [6]；脚本/测试 [8] [21]；运行器/CI [10] [22]；迁移 [20]；规范校验 [12] [13]；协作 [18] [19]。

可在当前检出直接核对：[发送流程](../../../internal/gui/app.go)、[HTTP 服务](../../../internal/service/request/request.go)、[认证](../../../internal/auth/auth.go)、[变量合并](../../../internal/gui/env.go)、[JavaScript API](../../../internal/script/api.js)、[导入转换](../../../internal/importer/openapi.go)、[本地存储](../../../internal/storage/storage.go)、[加密快照](../../../internal/syncing/sync.go)、[同步界面](../../../internal/gui/sync.go)、[GraphQL 编辑器](../../../internal/gui/graphql.go)、[WebSocket](../../../internal/websocket/client.go)、[程序入口](../../../cmd/reva/main.go)。

**迁移实际边界**：现行 Insomnia 原生导出是 v5 YAML，也能导入旧 v4 JSON；Reva 当前两种都不直接支持。HAR/curl 可转移一部分请求，规范可使用双方接受的 OpenAPI JSON；脚本、环境、mock、响应历史不保证保留。官方没有在本次资料中给出原生导出 Postman 的承诺，不据此编造无损迁移路径。[20]

## 六个真实功能议题

元数据取自 GitHub 查询；保留精确英文标题、原始标签与 UTC 更新时间。交付判定另外核对文档、源码或发布说明，不能用 closed 代替 shipped。

### #1638 — [FEATURE REQUEST] Ability to save and work on a collection/doc in local file system

- 议题：https://github.com/Kong/insomnia/issues/1638
- 状态：`open`；标签：`C-improvement`、`S-verified`；更新：`2024-08-19T08:21:29Z`。
- 用户需要：团队希望把请求按文件放在已有代码目录，用现有 Git 与 CI 管理，而非只控制应用内部数据库位置。
- 交付判定：开放提案；现有 Git Sync 是相关能力，未验证其满足任意路径直接打开编辑的原诉求。
- 交付依据：[18] [23]。

### #1784 — [Feature Request] Shared Fragments

- 议题：https://github.com/Kong/insomnia/issues/1784
- 状态：`open`；标签：`needs-triage`、`Feature Request`；更新：`2025-02-25T14:09:42Z`。
- 用户需要：多个 GraphQL 查询复用 fragment，修改一次即可更新全部请求，减少复制粘贴。
- 交付判定：开放提案；维护者在 2025-02-25 说明可用环境变量复用，多行编辑仍不直观；未核实原生共享 fragment 管理交付。
- 交付依据：[24] [7]。

### #3294 — [FEATURE REQUEST] Looping through a Data File like in the Postman Collection Runner

- 议题：https://github.com/Kong/insomnia/issues/3294
- 状态：`closed`；标签：无标签；更新：`2024-11-19T07:50:41Z`。
- 用户需要：从 CSV/JSON 的每行读取设备 ID 和配置，按顺序批量调用同一业务流程。
- 交付判定：已核实当前交付；2024-11-19 维护者称 GUI/CLI 已交付，现行 CLI 文档明确列出 CSV/JSON 参数，13.4 发布说明还有 CSV 引号解析修复。
- 交付依据：[10] [9] [4]。

### #281 — [Feature Request] Swagger (OpenAPI) import

- 议题：https://github.com/Kong/insomnia/issues/281
- 状态：`closed`；标签：无标签；更新：`2019-03-18T18:12:13Z`。
- 用户需要：从既有 Swagger/OpenAPI 定义生成可发送请求，避免手工重建全部端点。
- 交付判定：已核实当前交付；历史曾标为 #367 重复项，2019-02-07 按完成关闭；当时评论仍反映 OAS3 限制，不能据关闭日期宣称所有格式已支持，当前规范/导入文档确认现行能力。
- 交付依据：[12] [20] [26]。

### #203 — [Feature] Per-request Variables

- 议题：https://github.com/Kong/insomnia/issues/203
- 状态：`closed`；标签：`N-discussion`；更新：`2021-06-30T23:35:49Z`。
- 用户需要：给单个请求提供路径/深层 JSON 占位值，避免为临时参数修改共享环境。
- 交付判定：已关闭但原提案交付未核实；2021-06-30 被标为 completed 并锁定，所读页面未提供可核对的合并 PR/版本链。当前请求文档有 Path Parameters，但不足以证明原提议的通用请求变量面板完整交付。
- 交付依据：[27] [5]。

### #1389 — [Feature Request] Allow local encryption of passwords/user selected data

- 议题：https://github.com/Kong/insomnia/issues/1389
- 状态：`open`；标签：`N-discussion`、`needs-triage`、`Feature Request`；更新：`2025-02-13T16:15:32Z`。
- 用户需要：本地机器上的证书密码等敏感字段应可选择加密；云端 E2EE 不能保护本地明文。
- 交付判定：开放提案、部分相关能力已实现；Secret 变量有本地加密，但只覆盖私有全局子环境，不能据此宣称证书密码或任意字段全被加密。
- 交付依据：[28] [7]。

## 给 Reva 的三个可拆分改进

优先级按当前本地免账户定位判断，没有为追求功能数量提出 P0。以下是后续独立 PR 建议，本 PR 只研究文档。

### 1. 本地串行集合回归命令（P1）

- 用户场景：开发者修改接口后，在本机和 CI 对同一集合运行十个请求，任一断言失败应阻止提交。
- 现有缺口：已有 HTTP 服务和 pm.test，但只有 GUI 单请求发送，没有可复现的集合运行及用户侧失败退出码。
- 最小 PR：新增 cmd/reva-run 和无 GUI 的串行执行服务，只读原生集合/指定环境，复用 auth.Resolve、变量、选项、RunPre/RunTests 与 Requests.Send；先固定树遍历顺序，每请求独立变量快照，支持取消/超时，输出不含请求正文或凭据的 JSON 汇总，失败返回非零。前置脚本错误阻止该请求发送，明确它与现有 GUI 的差异；不在首版引入 CSV、调度、并行或持久化链式变量，也不默认写历史。
- 依赖：先从 App.Send 中明确可共享的发送顺序和错误约定；复用 service.RequestService、model、auth、vars、script，保持 Go/Viva 桌面入口独立，无新增运行时依赖。
- 验收：httptest 记录三个端点的实际到达顺序；中间响应触发 pm.test 失败时汇总含请求名、通过/失败数并返回非零，未被选中的端点从未收到请求；无限循环前置脚本超时且对应端点未收到请求，取消停止后续请求，运行前后集合/环境/历史文件字节不变，结果文件不含测试凭据。

### 2. 发送前变量来源与覆盖预览（P1）

- 用户场景：同名 baseUrl 在集合与环境中都存在，开发者切换到生产环境前需要知道请求到底发往哪里。
- 现有缺口：VarsFor 已有明确优先级，但调用者只得到扁平 KV；未知变量保留原文，现有提示主要覆盖 URL/header/auth，缺少统一来源视图。
- 最小 PR：增加纯 Go 的带来源解析结果，展示集合、当前环境、被覆盖值与未解析项；在现有请求/环境界面按需显示最终 URL 的只读预览。来源列表里的认证字段和值默认隐藏；预览仅计算且不执行脚本/网络，不新增持久化字段或私有环境存储。脚本可改写发送数据，界面注明预览在脚本执行前。
- 依赖：让 VarsFor 和预览共用同一解析规则；沿用 vars.Apply/Unresolved、现有 GUI 主题与中英 i18n，保持旧数据格式。
- 验收：无真实窗口的 GUI 测试切换两个环境，验证生效 URL/来源随之改变；禁用环境行退回集合值，空值覆盖和未知变量准确显示；验证预览没有调用发送服务、没有执行计数脚本、没有写磁盘，认证字段不以明文进入快照。

### 3. 受限 OAuth2 客户端凭据取令牌（P2）

- 用户场景：调用内部机器到机器 API 时，用户反复用另一个工具换取短时 access token，再粘贴 Bearer。
- 现有缺口：AuthType 只支持 Basic/Bearer/API key 与继承，没有 token endpoint 交互；手动 token 可用但易过期。
- 最小 PR：先做按需的 client_credentials 取令牌动作，用户配置 token URL、scope 与客户端凭据，令牌只放内存并用于已有 Bearer 发送；不自动后台刷新，不把 token 写入集合/环境/历史，不引入浏览器登录、PKCE、组织账户或通用 OAuth 平台。凭据处理和请求发送分离，关闭/重启即清除会话。
- 依赖：先约定会话令牌覆盖 Bearer 的优先级、历史保留模板而不保留令牌的行为；token 请求使用独立有超时且禁止跨源重定向的 HTTP 客户端，GUI 用 ui.Effect 与双语错误；需要独立设计本地凭据输入，不把现有掩码解释为 vault。
- 验收：httptest 验证表单 grant_type/scope、令牌响应与错误处理；两台本地测试服务器模拟跨源重定向，第二台不能收到 client secret；服务器延迟时可取消；使用短过期令牌后要求重新获取，重建 App 后内存 token 消失，磁盘集合/历史和错误信息均不出现测试 secret/token。

### 更适合接入专门工具的边界

OpenAPI 风格规则继续交给 Spectral，自动生成契约边界/反例测试交给专业 schema 工具；Reva 保留明确的 JSON 导入边界即可，不在 Go 桌面里打包 Node 工具链。负载/容量/百分位 SLO 交给 k6 等压测工具，集合重复执行不能替代压测。mock 优先保存响应并与独立服务配合，协议复杂的 gRPC 调试也可使用专门客户端；本研究不审计这些外部替代工具的版本或许可，选型须另做核对，更不会复制 Mockbin 的商业许可代码。

## 验证记录

只新增本 Markdown 和 JSON；没有新增行为测试、源文件、图标或依赖。README、AGENTS、onboarding PR 说明及上述实际代码已读；任务明确说明的 TASK.md 与本地 Viva skill 缺失不妨碍本次文档任务。

以下命令在本工作树执行，进程内设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`、`GOSUMDB=off`，全部退出 0；前面三项输出为空：

```text
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
ok  	github.com/runbrick/reva/internal/script	1.156s
?   	github.com/runbrick/reva/internal/service	[no test files]
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	1.562s
ok  	github.com/runbrick/reva/internal/service/store	1.065s
ok  	github.com/runbrick/reva/internal/storage	1.841s
ok  	github.com/runbrick/reva/internal/syncing	1.851s
ok  	github.com/runbrick/reva/internal/update	1.540s
ok  	github.com/runbrick/reva/internal/vars	0.868s
ok  	github.com/runbrick/reva/internal/websocket	1.360s
Exit: 0
```

JSON 已按约定顶层字段解析，Markdown/JSON 的条目和 issue 元数据一致；`git diff --check` 与仅两文件改动检查在提交前核对。

## 局限与证据范围

- 研究采用实际联网读取的官方 README、源码、文档、定价、发行说明和公开 issue；未安装/运行 Insomnia、未验证订阅账户权益或云端服务，不报告启动/内存/吞吐基准。
- 官方文档/定价和 issue 可变；源码固定至 51e449474b832760d6cdd1e47d53b572d2b24ddf，已发布版本记录为 core@13.4.0，不能把 develop 中每项代码等同于该版本已发布行为。Mockbin self-hosted 的 README/LICENSE 取自可变分支，明确保留其独立许可边界。
- 最初按不存在的 S:feature 标签查询返回零项，随后仅一次有效 issue 列表查询：repo:Kong/insomnia is:issue feature in:title，按评论数取前一百项（总六百六十一项）；逐页阅读所选六项，不声称覆盖全部需求或近期活跃度。部分长讨论被 GitHub 折叠，未读取全部评论。
- 所选议题三项开放、三项关闭；labels/state/updated_at 取查询时 GitHub 返回的元数据。关闭状态本身不视作已交付，#203 的原始提案交付链未核实；未查询私有路线图或内部 INS 工单。
- README 将 Git Sync 概括为 premium，而现行套餐/存储页列出 Essentials 三人配置；报告采用更具体的现行限制，未依据价格页图标推断每一项权益或固定价格。
- Reva 的缺口结论来自指定基线的模型、生产入口、服务、GUI 与脚本检查，表述为未找到相应产品入口；未实现任何建议、未改变源代码/依赖、未新增行为测试。TASK.md 与本地 Viva skill 缺失，任务已明确允许文档研究继续。
- 四项仓库检查使用当前本地缓存并设 GOPROXY=off、GOSUMDB=off；测试执行未访问互联网，未安装依赖。联网研究不属于测试。

## 编号证据来源

1. [Insomnia 官方 README：定位、存储与账户边界](https://raw.githubusercontent.com/Kong/insomnia/51e449474b832760d6cdd1e47d53b572d2b24ddf/README.md)
2. [Insomnia 主仓库 Apache-2.0 许可证](https://raw.githubusercontent.com/Kong/insomnia/51e449474b832760d6cdd1e47d53b572d2b24ddf/LICENSE)
3. [桌面包依赖：Electron、React、HTTP 与 gRPC](https://raw.githubusercontent.com/Kong/insomnia/51e449474b832760d6cdd1e47d53b572d2b24ddf/packages/insomnia/package.json)
4. [已发布版本 core@13.4.0，2026-10-29](https://github.com/Kong/insomnia/releases/tag/core%4013.4.0)
5. [请求类型与编辑器](https://developer.konghq.com/insomnia/requests/)
6. [请求认证支持列表](https://developer.konghq.com/insomnia/request-authentication/)
7. [变量优先级、加密秘密与保密标记](https://developer.konghq.com/insomnia/environments/)
8. [前后置脚本、异步请求与内置库](https://developer.konghq.com/insomnia/scripts/)
9. [集合运行器：顺序、迭代、延迟](https://developer.konghq.com/how-to/use-the-collection-runner/)
10. [集合 CLI：数据文件、报告与非交互选项](https://developer.konghq.com/inso-cli/reference/run_collection/)
11. [Inso CLI 命令总览](https://developer.konghq.com/inso-cli/)
12. [规范编辑、预览与自定义 lint](https://developer.konghq.com/insomnia/api-specs/)
13. [Spectral lint 实际实现](https://raw.githubusercontent.com/Kong/insomnia/51e449474b832760d6cdd1e47d53b572d2b24ddf/packages/insomnia-inso/src/commands/lint-specification.ts)
14. [云端与自托管 mock](https://developer.konghq.com/insomnia/mock-servers/)
15. [自托管 mock 部署：Docker 与 Redis](https://developer.konghq.com/insomnia/self-hosted-mocks/)
16. [Mockbin 自托管分支明确声明非开源](https://raw.githubusercontent.com/Kong/insomnia-mockbin/self-hosted/README.md)
17. [Mockbin 自托管许可证：Kong Software License Agreement](https://raw.githubusercontent.com/Kong/insomnia-mockbin/self-hosted/LICENSE)
18. [本地、Scratch Pad、云端与 Git 存储及套餐限制](https://developer.konghq.com/insomnia/storage/)
19. [官方套餐对比](https://insomnia.rest/pricing)
20. [导入导出与 v5 YAML 文件格式](https://developer.konghq.com/insomnia/import-export/)
21. [测试能力与传统单元测试弃用计划](https://developer.konghq.com/insomnia/test/)
22. [CLI 实现：CI 参数、数据文件和失败退出](https://raw.githubusercontent.com/Kong/insomnia/51e449474b832760d6cdd1e47d53b572d2b24ddf/packages/insomnia-inso/src/cli.ts)
23. [功能议题 #1638：[FEATURE REQUEST] Ability to save and work on a collection/doc in local file system](https://github.com/Kong/insomnia/issues/1638)
24. [功能议题 #1784：[Feature Request] Shared Fragments](https://github.com/Kong/insomnia/issues/1784)
25. [功能议题 #3294：[FEATURE REQUEST] Looping through a Data File like in the Postman Collection Runner](https://github.com/Kong/insomnia/issues/3294)
26. [功能议题 #281：[Feature Request] Swagger (OpenAPI) import](https://github.com/Kong/insomnia/issues/281)
27. [功能议题 #203：[Feature] Per-request Variables](https://github.com/Kong/insomnia/issues/203)
28. [功能议题 #1389：[Feature Request] Allow local encryption of passwords/user selected data](https://github.com/Kong/insomnia/issues/1389)
