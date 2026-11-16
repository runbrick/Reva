# Hoppscotch：对 Reva 的功能研究与最小改进建议

> 本地文档 PR，未推送、未创建远程 PR、未合并。仅新增本报告与同名 JSON，所有建议均为后续工作。

## 结论与研究基线

Hoppscotch 是以 Web 为中心，覆盖 Tauri 桌面和 CLI 的 API 开发生态；对 Reva 最有价值的是数据驱动回归、会话秘密隔离和可认证流式调试，而非复制云端协作服务。

- 检索记录截点：`2026-11-16T14:30:47Z`（UTC）；主机实际观测日期为 2026-11-16，采集约在 14:26–14:30 UTC。
- Reva 基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`，由本工作树开始时的 `git rev-parse HEAD` 得到。
- 上游源码固定为 `55908a76a1d0555c9bbb5d74244c14173df95b06`；最新正式发布为 2026.10.1，发布时间 2026-11-05 [30]。源码快照与已发布能力分别说明，不把主分支自动当成已发布。
- 主要分类：`desktop-client`；实际产品同时包括 Web 与 CLI。事实状态使用“已实现”“开放提案”“重复关闭”“已核实交付”区分。

## 定位、架构与许可边界

上游源码快照 55908a76a1d0555c9bbb5d74244c14173df95b06；检索时最新正式版 2026.10.1（2026-11-05）。pnpm 多包仓库，Vue/TypeScript 共享前端，Tauri 桌面与 Node CLI，NestJS/Prisma/PostgreSQL 后端支撑协作；浏览器通过 Agent/扩展/代理处理 CORS。Reva 是本地免账号 Go/Viva 原生客户端，适合保留单机文件模型。[3–6,29,30]

**许可结论：** 仓库根及官方社区版为 MIT，属于 OSI 许可；企业增值功能需商业许可。公开后端包另有 UNLICENSED 元数据冲突，复用前应向上游澄清，不能只依赖仓库徽章。 证据另见来源 [5,7,8]。 仓库地址核对为 [hoppscotch/hoppscotch](https://github.com/hoppscotch/hoppscotch)，根 LICENSE 是标准 MIT 正文 [1,2]。没有把公开源码、商业产品、托管服务视作同一个许可对象。

| 范围 | 已核对结论 | 对 Reva 的含义 |
| --- | --- | --- |
| 社区自托管 | 官方称 CE 为 MIT、可商用；支持工作区邀请和成员角色 [7]。公开后端包的 `license: UNLICENSED` 与此冲突 [5]，本次无法确定是元数据遗留还是另有意图。 | 研究产品行为可以继续；若后续复用后端代码，应先向上游澄清，不能仅凭根徽章做逐包授权判断。 |
| 企业自托管 | 官方明确增值功能附商业许可，含 SAML/OIDC、审计、高级管理等 [7,8]。 | 不称其为完整 MIT 或 OSI 开源版本，不把企业身份治理列入 Reva 的最小功能。 |
| 云端 | 检索到的套餐页把个人请求/脚本/runner 列在 Free，把团队实时协作、Mock、文档发布列在 Business，企业身份和审计另列 Enterprise [9]。 | 云端是否免费不能从 README 的“无限团队”等宣传推断；社区自托管能力不能直接推算云端免费权益。 |

## 已实现且有辨识度的能力

以下十项只列现有能力；每项中的限制不是已交付承诺。功能入口与限制优先采用固定源码、已合并 PR 和 release，动态官网文档作为补充。

1. **多协议实时调试（已实现）**：除 REST 外提供 WebSocket、SSE、Socket.IO、MQTT 专用界面；SSE 存在认证与自定义头等未交付诉求，不能把所有 REST 能力外推给实时协议。 [主要证据](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/README.md)。
2. **GraphQL 内省与模式浏览（已实现）**：实际调用 getIntrospectionQuery 与 buildClientSchema，提供类型文档和模式导航；比仅编辑 query/variables 更深入。[10] [主要证据](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/helpers/graphql/connection.ts)。
3. **继承认证与 OAuth2 授权流程（已实现）**：请求可继承集合或文件夹认证，支持 Basic、Bearer、API Key 和 OAuth2；授权码可用 PKCE。官方明确桌面 OAuth2 尚未完整支持，集合运行自动取令牌也不能视为已交付。 [主要证据](https://docs.hoppscotch.io/documentation/features/authorization.md)。
4. **分层变量与秘密值隔离（已实现）**：使用 `<<name>>`；请求、集合、预定义、环境、全局作用域各有优先级，秘密变量值不上传、不随环境导出。此规则不等于已证明本地加密，也不证明脚本可设置秘密变量。 [主要证据](https://docs.hoppscotch.io/documentation/features/variables.md)。
5. **预请求与响应测试脚本（已实现，部分接口属实验）**：提供旧 pw 与实验 hopp 接口及 pm 兼容层；支持变量、请求变更和断言。实验沙箱为渐进推出且可能破坏兼容；源码 jsonSchema 辅助函数主要检查 type/required/properties，不能声称完整 JSON Schema 合规。[15] [主要证据](https://docs.hoppscotch.io/documentation/features/scripts.md)。
6. **GUI 数据驱动集合运行器（已实现）**：2026.8.0 已发布 CSV/JSON 数据集、逐行迭代、请求子集选择和结果 JSON 导出；当前 GUI 数据文件上限 2 MiB，预览前 50 行；有延迟、停止、保留响应和变量控制。[16–18,31] [主要证据](https://github.com/hoppscotch/hoppscotch/pull/6513)。
7. **本地 CLI 与 CI 结果（已实现）**：hopp test 可直接运行本地集合及环境，不要求云账号；支持迭代、延迟、JUnit XML 与断言失败非零退出码。远程集合访问另需 PAT，当前 CLI 迭代数据只接受 CSV，不能套用 GUI 的 JSON 支持。[21] [主要证据](https://docs.hoppscotch.io/documentation/clients/cli/overview.md)。
8. **多工具迁移与 OpenAPI 导出（已实现）**：支持 Hoppscotch、Postman、Insomnia、HAR、OpenAPI 导入及 curl；2026.5.0 已发布 OpenAPI 3.1 导出。Postman 脚本导入是需同意的实验功能，不能承诺无损迁移。[22,23,25] [主要证据](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.5.0)。
9. **共享工作区与角色（已实现）**：社区自托管支持邀请成员、角色和共享资源；云端页面把团队协作列入 Business。Owner/Editor/Viewer 有文档；2026.8.0 声明统一 REST/GraphQL 工作区，旧工作区文档仍写共享仅 REST，协议权限细节需部署验证。[9,18,26] [主要证据](https://docs.hoppscotch.io/documentation/self-host/getting-started.md)。
10. **由响应示例建立 Mock（已实现）**：公开后端有 Mock 模块，按请求匹配示例并返回状态、头、正文及配置延迟；云端套餐把 Mock 列入 Business。formatExampleResponse 直接返回 responseBody，动态替换的文档承诺未获当前源码证实。[9,27,32] [主要证据](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-backend/src/mock-server/mock-server.service.ts)。

## 与当前 Reva 的直接比较

先读取了 README、AGENTS.md、`docs/onboarding-prs.md`，再核对下表模型、入口与实际实现；没有用“README 未提及”替代代码调查。Reva 的本地 JSON 是明文，AES-GCM 备份是另一条可选路径，输入遮罩不等于凭证库。

| 领域 | Hoppscotch | Reva 当前实现 | 可取方向 |
| --- | --- | --- | --- |
| 协议与 GraphQL | REST、WebSocket、SSE、Socket.IO、MQTT；GraphQL 有内省和模式文档。[3,10,11] | 已有 HTTP、GraphQL query/variables 编辑和 RFC6455 WebSocket；未发现 SSE 流式面板、Socket.IO、MQTT 或 gRPC 入口。 | 优先补可认证的 HTTP 事件流；专用消息协议使用外部专业客户端，避免同时扩张协议栈。 |
| 认证 | 继承认证与 OAuth2/PKCE；桌面流程有官方限制，运行前自动取令牌仍为提案。[12] | 已有请求/文件夹/集合 Basic、Bearer、API Key 与继承屏障；跨源重定向剥离凭证；无 OAuth2 生命周期。 | 保留现有继承与安全语义；未来独立做本地 OAuth2 授权，不把桌面或 runner 的自动续期视为简单附带项。 |
| 变量与本地秘密 | `<<name>>` 多层变量；秘密值不进入服务端与环境导出。[13] | `{{name}}`；集合变量后接活动环境，后者覆盖前者；脚本修改是本次发送的复制值；KV 无秘密标记。 | 增加纯内存会话秘密覆盖层；无需账号或密钥托管服务。 |
| 脚本与模式校验 | 预请求/响应脚本、实验新沙箱及 Postman 兼容；已读 jsonSchema 路径仅实现有限校验。[14,15] | Otto 限时默认两秒；pm.test/expect、请求头变更、变量和响应访问；拒绝异步测试；无 jsonSchema API，也未找到自动 OpenAPI 响应校验。 | 先抽出可复用执行服务；完整契约校验和生成式反例测试接入专业工具，不自行声称模式标准全兼容。 |
| 集合运行与 CI | GUI 数据驱动；CLI 本地文件、CSV 迭代、JUnit、失败退出码。[17–21,31] | 模型持久化集合与测试脚本；发送/测试编排在 GUI；cmd 仅启动应用；CI 是构建 Reva 本身，未发现用户集合 runner 命令。 | 新增无窗口、无账号的顺序集合运行命令；避免让 GUI 运行器先成为独立执行引擎。 |
| 导入导出 | 多格式导入，实验脚本迁移，OpenAPI 3.1 导出已发布。[22–25] | 已有原生、Postman v2、HAR、curl、Swagger2/OpenAPI3 JSON 与 URL 导入，导入前审核告警；Postman 脚本省略；集合导出为原生 JSON；没有 OpenAPI 导出。 | 维持迁移损失可见；后续若增加标准导出，明确丢失脚本/继承语义；不能直接把原生 JSON 交给 hopp test。 |
| Mock 与契约 | 公开后端 Mock 按示例返回响应；动态替换尚未证实，模式断言能力有限。[15,27,28,32] | 仅有测试用 fake 服务，未发现供最终用户配置、监听的 Mock 服务器；OpenAPI 导入生成模板且省略响应与约束。 | 优先与独立 Mock 服务及契约测试工具通过 URL/规范文件协作；fake 测试替身不能计作产品 Mock。 |
| 性能与网络可观测性 | 顺序 runner 给总耗时、均值；详细网络阶段时间仍有功能诉求；本次未发现可据以承诺负载生成能力的证据。[16] | 已有 DNS/连接/TLS/发送/首字节等待/下载时间轴、响应体上限与截断状态；不是负载生成器。 | 保留原生网络诊断优势；并发压力、分位数和分布式负载交给 k6 等专业工具，runner 只负责功能回归。 |
| 协作、架构与数据 | Vue/TypeScript 共享前端、Tauri 桌面、Node CLI；后端 NestJS/Prisma/PostgreSQL 支撑共享与同步；浏览器需处理 CORS；企业 SSO/审计为商业能力。[4–9,26,29] | Go/Viva 自绘、无 WebView；明文 JSON 在 REVA_HOME/用户目录，独立离线使用；已有加密备份与 WebDAV/S3/GitHub Gist/Gitee 同步；未发现成员角色和多人实时编辑。 | 维护免账号和可审阅本地文件；协作先用去秘密的文件与 Git，组织 IAM/SSO 适合对接现有平台。 |

Reva 源码定位（均相对本报告目录，可点击；符号名是审查入口）：

- **协议与 GraphQL**：[`internal/gui/graphql.go`](../../../internal/gui/graphql.go) `initGraphQLBody/graphQLBodySection`；[`internal/websocket/client.go`](../../../internal/websocket/client.go) `Dial`；[`internal/gui/websocket.go`](../../../internal/gui/websocket.go)；[`internal/service/request/request.go`](../../../internal/service/request/request.go) `Send`；[`cmd/reva/main.go`](../../../cmd/reva/main.go)。
- **认证**：[`internal/model/model.go`](../../../internal/model/model.go) `AuthTypes`；[`internal/auth/auth.go`](../../../internal/auth/auth.go) `Resolve`；[`internal/service/request/request.go`](../../../internal/service/request/request.go)；[`internal/service/request/auth_test.go`](../../../internal/service/request/auth_test.go)。
- **变量与本地秘密**：[`internal/vars/vars.go`](../../../internal/vars/vars.go) `Lookup/Apply`；[`internal/gui/env.go`](../../../internal/gui/env.go) `VarsFor`；[`internal/script/script.go`](../../../internal/script/script.go) `RunPre/newRunner`；[`internal/model/model.go`](../../../internal/model/model.go) `KV`；[`internal/gui/collections.go`](../../../internal/gui/collections.go) `exportCollection`。
- **脚本与模式校验**：[`internal/script/script.go`](../../../internal/script/script.go) `DefaultTimeout/RunPre/RunTests`；[`internal/script/api.js`](../../../internal/script/api.js)；[`internal/gui/scripts.go`](../../../internal/gui/scripts.go) `prepareSend/afterResponse`；[`internal/importer/openapi.go`](../../../internal/importer/openapi.go)。
- **集合运行与 CI**：[`internal/model/model.go`](../../../internal/model/model.go) `Collection/Request`；[`internal/gui/app.go`](../../../internal/gui/app.go)；[`internal/gui/scripts.go`](../../../internal/gui/scripts.go)；[`cmd/reva/main.go`](../../../cmd/reva/main.go)；[`cmd/reva/services.go`](../../../cmd/reva/services.go)；[`.github/workflows/ci.yml`](../../../.github/workflows/ci.yml)。
- **导入导出**：[`internal/importer/postman.go`](../../../internal/importer/postman.go)；[`internal/importer/har.go`](../../../internal/importer/har.go)；[`internal/importer/curl.go`](../../../internal/importer/curl.go)；[`internal/importer/openapi.go`](../../../internal/importer/openapi.go)；[`internal/importer/fetch.go`](../../../internal/importer/fetch.go)；[`internal/gui/importer.go`](../../../internal/gui/importer.go)；[`internal/gui/collections.go`](../../../internal/gui/collections.go) `exportCollection`。
- **Mock 与契约**：[`internal/service/fake/fake.go`](../../../internal/service/fake/fake.go)；[`internal/importer/openapi.go`](../../../internal/importer/openapi.go)；[`internal/model/model.go`](../../../internal/model/model.go)；[`cmd/reva/services.go`](../../../cmd/reva/services.go)。
- **性能与网络可观测性**：[`internal/service/request/trace.go`](../../../internal/service/request/trace.go)；[`internal/service/request/request.go`](../../../internal/service/request/request.go) `DefaultMaxBodyBytes`；[`internal/model/model.go`](../../../internal/model/model.go) `Response/Phase`；[`internal/gui/response.go`](../../../internal/gui/response.go)。
- **协作、架构与数据**：[`README.md`](../../../README.md)；[`go.mod`](../../../go.mod)；[`cmd/reva/main.go`](../../../cmd/reva/main.go)；[`internal/storage/storage.go`](../../../internal/storage/storage.go)；[`internal/syncing/sync.go`](../../../internal/syncing/sync.go) `Seal`；[`internal/gui/sync.go`](../../../internal/gui/sync.go)；[`internal/s3/s3.go`](../../../internal/s3/s3.go)；[`internal/gist/gist.go`](../../../internal/gist/gist.go)。

三个容易失真的比较点：

- **运行器不是压测器。** 顺序迭代与平均响应时间服务于回归和批量数据操作；没有证据支持把 Hoppscotch 或 Reva 称为分布式负载平台。Reva 已有网络阶段时间轴，不能反向把竞品的开放提案算作 Reva 的缺口。
- **格式支持不等于无损互通。** Hoppscotch 的脚本迁移需要实验同意；Reva 有审核告警，Postman 脚本明确省略。Reva 原生 JSON 与 Hoppscotch 集合不是同一 schema，不能直接给 `hopp test`。
- **Mock 与模式校验需看具体实现。** 已核对的 Mock `formatExampleResponse` 直接返回静态正文，文档所述动态替换未验证；脚本 `jsonSchema` 只看到有限的类型、必填与递归属性检查，不是完整标准校验或 OpenAPI 契约测试。

## 功能议题：真实需求与交付证据

一次读取 `state=all&labels=feature&per_page=100&sort=updated&direction=desc` 列表，排除 PR，再读取所选议题正文、可得评论和对应发布/源码。下列 8 项含 5 个开放、3 个关闭（其中 1 个重复），是近期更新的功能样本，不能代表全部用户需求或完整路线图；标题、状态、标签及更新时间均保持实际记录。

### [#3145 [feature]: REST features for SSE](https://github.com/hoppscotch/hoppscotch/issues/3145)

- 状态：`open`；标签：`feature`；更新：`2026-11-15T14:33:59Z`。
- 用户需要：SSE 不只是打开连接，还需要参数、认证、请求头、脚本；评论明确需要 POST 后逐步显示流。
- 交付判断：**开放提案；完整 REST 配置未验证交付**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/3145#issuecomment-2887695422)；[证据 2](https://github.com/hoppscotch/hoppscotch/issues/3145#issuecomment-6896166141)。

### [#4528 [feature]: Set/unset secret variables from scripts](https://github.com/hoppscotch/hoppscotch/issues/4528)

- 状态：`open`；标签：`feature`；更新：`2026-11-12T18:52:04Z`。
- 用户需要：多人共用环境时，登录脚本得到的令牌应归当前用户私有，避免写入团队共享变量。
- 交付判断：**开放提案；秘密变量本身已实现，脚本设置/删除秘密值未验证交付**。
- 核验依据：[证据 1](https://docs.hoppscotch.io/documentation/features/variables.md)；[证据 2](https://github.com/hoppscotch/hoppscotch/issues/4528)。

### [#5478 [feature]: Automatic OAuth 2.0 Token Generation on Collection Run](https://github.com/hoppscotch/hoppscotch/issues/5478)

- 状态：`open`；标签：`feature`；更新：`2026-09-24T12:15:44Z`。
- 用户需要：集合执行前统一生成新 OAuth2 令牌，避免手动点击或每条请求重复授权。
- 交付判断：**开放提案；作者声称提供实现不等于已合并或已发布**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/5478#issuecomment-3521728779)；[证据 2](https://github.com/hoppscotch/hoppscotch/pull/5575)；[证据 3](https://docs.hoppscotch.io/documentation/features/authorization.md)。

### [#5163 [feature]: Collection runner via GUI with ability to import files and choose which requests should be run](https://github.com/hoppscotch/hoppscotch/issues/5163)

- 状态：`closed`；标签：`feature`；更新：`2026-08-31T05:54:07Z`。
- 用户需要：QA 用 CSV/JSON 批量造数据，并选择集合中的请求子集，无需先使用 CLI。
- 交付判断：**已关闭；已核实交付**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/5163#issuecomment-5474331078)；[证据 2](https://github.com/hoppscotch/hoppscotch/pull/6513)；[证据 3](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.8.0)；[证据 4](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/components/http/test/RunnerModal.vue)。

### [#3821 [feature]: Export to OpenAPI format](https://github.com/hoppscotch/hoppscotch/issues/3821)

- 状态：`closed`；标签：`feature`；更新：`2026-05-28T14:46:51Z`。
- 用户需要：团队成员使用不同客户端，需要以 OpenAPI 标准交换接口，避免锁定私有集合格式。
- 交付判断：**已关闭；已核实交付 OpenAPI 3.1 导出**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/3821#issuecomment-4565258639)；[证据 2](https://github.com/hoppscotch/hoppscotch/pull/5880)；[证据 3](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.5.0)；[证据 4](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/helpers/import-export/export/openapi.ts)。

### [#6285 [feature]: Add file support to Collection Runner](https://github.com/hoppscotch/hoppscotch/issues/6285)

- 状态：`closed`；标签：`duplicate`, `feature`；更新：`2026-05-14T14:35:39Z`。
- 用户需要：运维批量更新需要由 CSV 文件驱动 GUI 集合运行，单纯已有 runner 仍无法迁移。
- 交付判断：**已关闭为重复；关闭时不代表交付，后由规范议题 #5163 的发布证据确认同类需求交付**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/6285#issuecomment-4451653469)；[证据 2](https://github.com/hoppscotch/hoppscotch/issues/5163)；[证据 3](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.8.0)。

### [#5980 [feature]: Mock Server — Variable resolution and dynamic response body substitution](https://github.com/hoppscotch/hoppscotch/issues/5980)

- 状态：`open`；标签：`feature`；更新：`2026-06-13T12:29:37Z`。
- 用户需要：Mock 应解析带变量的请求匹配条件，并在响应中反映路径、查询或请求体值。
- 交付判断：**开放提案；响应变量替换未验证交付，当前核对到的格式化函数返回静态正文**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/5980)；[证据 2](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-backend/src/mock-server/mock-server.service.ts)；[证据 3](https://docs.hoppscotch.io/documentation/features/mock.md)。

### [#6411 [feature]: Add detailed response timing breakdown similar to Postman](https://github.com/hoppscotch/hoppscotch/issues/6411)

- 状态：`open`；标签：`feature`；更新：`2026-06-28T11:20:45Z`。
- 用户需要：开发者需要区分 DNS、连接、TLS、TTFB 和下载耗时，以解释总延迟。
- 交付判断：**开放提案；未验证交付详细阶段时间轴，Reva 当前已有相关实现**。
- 核验依据：[证据 1](https://github.com/hoppscotch/hoppscotch/issues/6411)；[证据 2](https://docs.hoppscotch.io/documentation/features/runner.md)。

关闭议题的解释：#5163 与 #3821 均有“已合并 PR → 正式 release → 当前实现”证据链；#6285 的关闭原因是重复，只有追到 #5163 后的正式发布才能确认同类需求交付。#5478 评论只是作者声称 PR 可用，本次没有合并/发布证据，保留为开放提案；#4528 也不能由“已有秘密变量”反推出“脚本可以写秘密”。

## 推荐的三个后续 PR

优先级反映本地免账号方向下的收益与前置工作，不是竞品全部功能的实现顺序。本次不实现这些建议；应分别申请所有权和评审接口，保留纯 Go/Viva，不复制上游代码。

### 1. P0：增加纯内存会话秘密变量覆盖层

- **场景**：开发者将集合和环境放进 Git，访问令牌每次启动后手动填入，退出后不保留；共享集合只保留变量名。
- **现有缺口**：KV 没有秘密语义，输入遮罩不改变原生导出、磁盘和加密备份中的明文数据模型；历史只保证保留占位符，脚本写死秘密仍可能泄露。
- **最小 PR**：在 App 会话内维护独立覆盖表，通过独立的发送变量解析入口叠加在集合/环境之上；首个实现 PR 先支持手动填入和清空，不修改持久化模型。序列化路径永远接触不到覆盖值；UI 提示此为会话秘密，响应正文或用户显式打印仍需自行处理。
- **依赖**：复用 VarsFor/vars.Apply 与现有认证继承；中英 i18n、Viva 输入遮罩与 headless harness；核对历史、集合导出、Snapshot 与日志边界；无需新依赖或账号。
- **有意义的验收**：用 httptest 验证 `{{token}}` 解析后仅到达预期请求头；覆盖值优先且清空后回退；发送和导出后扫描临时 REVA_HOME 文件，并解密备份检查 Snapshot，均不含覆盖秘密；重建 App 后覆盖为空，原集合/环境保持原值。测试不打印秘密。

### 2. P1：新增原生集合的无窗口顺序回归命令

- **场景**：开发者在本地终端或 CI 对同一已保存集合执行登录后检查接口，失败时阻断流水线。
- **现有缺口**：已有发送器和 pm.test，但测试编排属于 GUI，没有集合级执行入口、可机读报告或用户测试退出码。
- **最小 PR**：先抽出纯 Go 单请求执行服务并让 GUI 复用；新增 `reva run <原生集合文件> --env <原生环境文件>`，按集合树顺序执行请求并沿用键值行的启停设置。首版输出 JSON 摘要并以断言失败/脚本错误/发送错误返回非零，可取消；不添加并发、CSV、JUnit 或 GUI 面板。将预脚本出错是否继续发送明确为 runner 策略并记录，与现有 GUI 行为分别验收。
- **依赖**：复用 request.Service、auth.Resolve、vars、script 与 RequestOptions.Resolve，明确定义命令行设置默认值；提取内部执行编排，避免依赖 Viva 初始化；明确变量只在一次运行中传递而不回写源文件；现有 RunTests 不返回修改值，跨请求令牌传递需显式增加结果契约。
- **有意义的验收**：httptest 构建登录与鉴权两端点，第一条脚本提取令牌仅供同次第二条使用；正确集合退出 0，错误断言/无限脚本/断网模拟均非零且摘要定位请求；取消后不发送下一条；不创建窗口、不加载远端账号、不改集合或环境文件。

### 3. P2：为 HTTP 增加可认证的 SSE 实时响应视图

- **场景**：调试 POST 的大模型流或需要 Bearer 的事件订阅，在连接结束前看到事件并能立即停止。
- **现有缺口**：已有 WebSocket，但 HTTP Send 在读取结束后才交回 Response；没有 SSE 事件增量面板，仅新增协议菜单不能满足该场景。
- **最小 PR**：建立单独的 Go 事件流服务，复用现有 HTTP 请求构造和认证规则，支持手动 GET/POST 的 text/event-stream、空行分帧、多行 data/id/event 与取消；通过 ui.Effect 向状态提交有界事件批次。首版只做当前会话显示，不自动重连、不持久化消息、不引入 Socket.IO/MQTT。
- **依赖**：共享 HTTP 构造、认证继承与重定向保护；独立接口及 fake；Viva Effect 事件交付方式须按实际 API 验证；中英文响应状态、明确事件数/单事件字节上限及溢出行为。
- **有意义的验收**：本地 httptest POST 服务先校验 Bearer，再 Flush 两个事件且保持连接；headless 测试在 EOF 前观察到首事件，多行 data 正确合并；取消及时关闭连接；跨源重定向不泄露认证；超限输入有明确截断或终止结果。

## 适合整合专业工具的边界

以下是设计建议，不声称已经做了兼容认证：

- **契约与模式测试**：保留 OpenAPI 源文件，使用 Schemathesis 等工具独立执行生成式契约测试；Reva 提供查看请求与重放入口。不要为模仿少量 `jsonSchema` 断言自制完整标准引擎。
- **负载测试**：用 k6 等工具管理并发、场景、分位数与 CI 阈值；Reva 保留单请求时间轴，未来可提供经审查的请求导出。集合 runner 的次数不等于负载模型。
- **Mock**：用独立 Mock 服务（如 WireMock/Mockoon）维护响应、匹配和故障场景，Reva 用环境变量切换 URL；不要把现有测试 fake 包包装成产品 Mock。
- **团队身份与实时协作**：确需成员角色、组织 SSO 或云端共享时，可在隔离项目使用 Hoppscotch 自托管/云端及其 CLI；需另做格式转换或继续维护标准规范。Reva 不必引入常驻后端和账号体系。

## 检查与交付范围

分支：`runbrick/research-hoppscotch`。本次新增两个 UTF-8 文档，无源码、测试、依赖或图标变更，无新增行为测试；没有请求修改其他所有者文件。提交哈希用 `git log -1 --format=%H -- docs/research/api-tools/hoppscotch.md` 获取，避免将提交自身哈希写进被提交文件。

在本工作树实际执行；仅子进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`。以下是退出码与输出尾部，测试未增加互联网访问：

```text
$ gofmt -l .
（输出为空）
Exit: 0
$ go vet ./...
（输出为空）
Exit: 0
$ go tool viva vet ./...
（输出为空）
Exit: 0
$ go test ./... -count=1
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	2.587s
ok  	github.com/runbrick/reva/internal/service/store	3.013s
ok  	github.com/runbrick/reva/internal/storage	2.412s
ok  	github.com/runbrick/reva/internal/syncing	2.619s
ok  	github.com/runbrick/reva/internal/update	0.537s
ok  	github.com/runbrick/reva/internal/vars	1.441s
ok  	github.com/runbrick/reva/internal/websocket	0.397s
Exit: 0
```

全套结果还包括 `internal/gui 11.499s`、`internal/i18n 0.045s` 均通过。JSON 解析、字段结构、议题元数据和本地链接另做校验，`git diff --check` 通过；最终提交仅含本报告与 JSON。

## 局限与不确定性

- 仅对指定 Reva 基线和上游源码快照做静态核对；未安装上游依赖、启动上游部署或实测性能，因此没有启动速度、内存或吞吐排名。
- GitHub 只取一次带 feature 标签、按更新时间降序的前 100 条列表，排除 PR；不是全部历史议题普查。选取 8 个实际功能议题，含 5 个开放、3 个关闭，其中 1 个重复关闭。
- GitHub 部分接口返回 403 限流；递归树和 #4528/#5324 评论接口未成功，随后使用官方提交归档与直接页面，没有反复刷新列表；议题元数据来自已取得的列表，未将无法取得的评论编造为证据。
- 官方文档为浮动页面，检索时间之后可能变化；runner 文档缺少已发布数据集细节，工作区文档旧限制与 2026.8.0 冲突，Mock 动态响应文档与当前已读源码不符，已在正文分别注明。
- 仓库根许可与官方社区版文档为 MIT，但后端 package.json 标注 UNLICENSED；未进行全部依赖/子包法律审计，也未取得企业商业许可全文，不把企业功能或仅源码可见内容统一称作 OSI 开源。
- 云端免费/Business/Enterprise 与社区自托管功能不是同一授权或套餐范围；不保证具体部署的开关、席位、配额、协议协作权限，也不引用易变价格作为决策依据。
- Reva 的未发现结论限于已读模型、入口、服务、GUI 与包搜索；只导入 OpenAPI 模板不等于完整契约验证，测试 fake 不等于产品 Mock，CI 构建项目不等于用户集合 runner。
- 建议是后续 PR 方案，本次仅新增文档，不新增行为测试，不修改 Go/Viva 架构。TASK.md 与本地 Viva 技能缺失按任务说明处理；未创建远程 PR、推送或合并。

## 编号证据来源

固定源码链接指向上文提交；官网和议题链接是检索时页面，之后可能更新。议题的标签/状态/更新时间保存在 JSON 中以便复核。

1. [官方规范仓库](https://github.com/hoppscotch/hoppscotch)
2. [仓库根 MIT 许可正文](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/LICENSE)
3. [上游功能、协议及企业版脚注](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/README.md)
4. [共享前端依赖与构建入口](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/package.json)
5. [后端技术栈及 UNLICENSED 元数据](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-backend/package.json)
6. [Tauri 桌面构建入口](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-desktop/package.json)
7. [社区版与企业版比较](https://docs.hoppscotch.io/documentation/self-host/getting-started.md)
8. [企业商业许可边界](https://docs.hoppscotch.io/documentation/self-host/enterprise-edition/getting-started.md)
9. [云端套餐边界，取回页面默认展示年度计费](https://hoppscotch.com/pricing)
10. [GraphQL 查询与模式浏览](https://docs.hoppscotch.io/documentation/features/graphql-api-testing.md)
11. [GraphQL 内省请求及模式构建源码](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/helpers/graphql/connection.ts)
12. [请求认证、继承、OAuth2 及桌面限制](https://docs.hoppscotch.io/documentation/features/authorization.md)
13. [变量优先级与秘密值导出规则](https://docs.hoppscotch.io/documentation/features/variables.md)
14. [脚本接口与实验状态](https://docs.hoppscotch.io/documentation/features/scripts.md)
15. [模式断言实际支持范围](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-js-sandbox/src/cage-modules/utils/chai-helpers.ts)
16. [集合运行配置及结果查看](https://docs.hoppscotch.io/documentation/features/runner.md)
17. [GUI 数据集、迭代与大小限制](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/components/http/test/RunnerModal.vue)
18. [已发布的数据驱动运行器和 REST/GraphQL 统一工作区](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.8.0)
19. [已合并的数据集、请求选择与运行结果导出实现](https://github.com/hoppscotch/hoppscotch/pull/6513)
20. [命令行运行、迭代、JUnit 与退出码](https://docs.hoppscotch.io/documentation/clients/cli/overview.md)
21. [CLI 数据文件只接受 CSV 的当前入口](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-cli/src/commands/test.ts)
22. [迁移格式与实验性 Postman 脚本导入](https://docs.hoppscotch.io/documentation/features/importer.md)
23. [OpenAPI 导出实现](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/helpers/import-export/export/openapi.ts)
24. [已发布的 OpenAPI 3.1 导出](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.5.0)
25. [已合并的 OpenAPI 3.1 导出实现](https://github.com/hoppscotch/hoppscotch/pull/5880)
26. [成员角色与仍含旧限制的工作区文档](https://docs.hoppscotch.io/documentation/features/workspaces.md)
27. [Mock 产品说明，部分动态能力待核实](https://docs.hoppscotch.io/documentation/features/mock.md)
28. [Mock 匹配与静态响应格式化实现](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-backend/src/mock-server/mock-server.service.ts)
29. [浏览器跨域、代理与本地 Agent 路径](https://docs.hoppscotch.io/documentation/features/interceptor.md)
30. [检索时最新正式发布记录](https://github.com/hoppscotch/hoppscotch/releases/tag/2026.10.1)
31. [集合迭代执行、数据变量及结果汇总](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-common/src/services/test-runner/test-runner.service.ts)
32. [公开后端注册 Mock 模块](https://github.com/hoppscotch/hoppscotch/blob/55908a76a1d0555c9bbb5d74244c14173df95b06/packages/hoppscotch-backend/src/app.module.ts)
33. [功能议题 #3145，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/3145)
34. [功能议题 #4528，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/4528)
35. [功能议题 #5478，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/5478)
36. [功能议题 #5163，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/5163)
37. [功能议题 #3821，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/3821)
38. [功能议题 #6285，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/6285)
39. [功能议题 #5980，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/5980)
40. [功能议题 #6411，标题与状态取自本次议题列表](https://github.com/hoppscotch/hoppscotch/issues/6411)
