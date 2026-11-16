# Bruno 对 Reva 的研究：文件协作、自动化与本地数据边界

> 检索完成：`2026-11-16T22:29:29+08:00`；Reva 基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`。分类：`desktop-client`；规范仓库：[usebruno/bruno](https://github.com/usebruno/bruno)。

## 定位与结论

Bruno 是文件优先的跨平台 API 开发与测试客户端，差异集中在可 Git 评审的项目、脚本和断言、可进入 CI 的 CLI。Reva 已有 HTTP、GraphQL、WebSocket、限时 JavaScript、认证继承及可选加密同步；应先补会话本地覆盖与最小执行管线，通过明确导出连接专业工具，保持免账号、纯 Go/Viva。

Electron 主进程处理网络、文件与 IPC，React 渲染层；monorepo 拆分 lang、schema、js、requests、converters、cli、sqlite、filestore 等包。安全脚本为 QuickJS，开发者模式为 Node vm。新建与导入集合默认 OpenCollection YAML，兼容 Bru；请求文件留在项目目录，但并非所有状态都只是文本：源码含 SQLite，v4.3.0 发布提到运行结果正文的文件存储。Reva 以 Go/Viva 直接调用 Go 服务与 ui.Effect，无 WebView，使用原子 JSON 持久化。 [4](https://github.com/usebruno/bruno/releases/tag/v4.3.0)[5](https://github.com/usebruno/bruno/blob/main/packages/bruno-electron/package.json)[6](https://github.com/usebruno/bruno/blob/main/packages/bruno-app/package.json)[16](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/quickjs/index.js)[17](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/node-vm/index.js)

## 许可证与社区／商业边界

**公共仓库许可证是 MIT，属于 OSI 开源，而非受限“源码可见”许可。** 此结论由根许可证和 GitHub 仓库元数据交叉核实，范围只覆盖公共源码，不自动覆盖商业增值产品的分发权益、依赖或商标；README 明确有商业版本，标识来自 OpenMoji，另采用 CC-BY-SA-4.0。[1](https://github.com/usebruno/bruno/blob/main/readme.md)[2](https://github.com/usebruno/bruno/blob/main/license.md)

官方定价页同时列 Open Source、Pro、Ultimate、Enterprise。基础 Git UI 自 3.0.0 免费，提交、推送、分支和冲突处理付费；桌面数据驱动与并行需 Pro/Ultimate，下载报告需 Ultimate；外部秘密管理器与整批 Postman 数据迁移另有 Ultimate 边界。**公共 MIT CLI 的报告输出不能套用桌面下载报告的付费限制。** 基础本地请求使用无需引入团队云账号；企业管理功能的账号要求不外推为社区基本请求要求。[3](https://www.usebruno.com/pricing)[8](https://docs.usebruno.com/git-integration/using-gui/intro)[9](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-cli/src/commands/run.js)[24](https://docs.usebruno.com/secrets-management/secret-managers/overview)[28](https://docs.usebruno.com/get-started/import-export-data/import-collections)[30](https://docs.usebruno.com/testing/automate-test/data-driven-testing)

## 十项已经实现的特色能力

### 1. 文件项目、YAML 与 Git 协作

**已实现；免费基础 Git UI 与付费高级操作分开。** v3.1.0 起新建和导入集合默认 OpenCollection YAML，仍支持 Bru；v4.1.0 起可迁移，一个集合不能混用 .bru/.yml。免费 Git UI 可初始化、克隆、文本差异与拉取；提交、推送、分支和冲突处理需 Pro/Ultimate。 [7](https://docs.usebruno.com/bru-lang/yaml-migration)[8](https://docs.usebruno.com/git-integration/using-gui/intro)

### 2. 公共 CLI 执行与机器报告

**已实现；v4.3.0 公共 MIT CLI 源码确认。** bru run 可运行请求或文件夹，选择环境、递归、标签过滤、失败即停，输出 JSON/JUnit/HTML 并省略敏感头和正文。桌面报告下载权益不能直接套用到这份 CLI；未将文档中的数据文件和并行参数算成公共 CLI 已核实能力。 [9](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-cli/src/commands/run.js)[11](https://docs.usebruno.com/bru-cli/run/reports)

### 3. 声明式断言与脚本结构校验

**已实现；官方操作及库文档确认。** Assert 表格可检查状态、响应字段、数组、头、耗时，含类型、字符串和数值运算；复杂测试使用 Chai。内置 tv4 可做 JSON Schema v4 校验，Ajv 需开发者模式；不等于完整 OpenAPI 契约覆盖系统。 [12](https://docs.usebruno.com/testing/tests/assertions)[13](https://docs.usebruno.com/testing/script/inbuilt-libraries)

### 4. 多层脚本与可选高权限模式

**已实现；安全模式默认。** 支持集合、文件夹、请求级前后 JavaScript、bru API、请求链；安全模式用 QuickJS，开发者模式允许外部 npm 包、文件系统和系统命令。CLI 需显式 --sandbox=developer，公共源码有 quickjs 与 node-vm 实现。 [15](https://docs.usebruno.com/get-started/configure/javascript-sandbox)[14](https://docs.usebruno.com/testing/script/overview)[16](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/quickjs/index.js)[17](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/node-vm/index.js)

### 5. gRPC 流式调用及多协议编辑

**已实现；gRPC 文档称自 2.10.0 提供。** gRPC 有一元、客户端流、服务端流、双向流，支持 proto、反射、模板与消息时间线。产品表另列 HTTP/REST/GraphQL，源码测试含原始 WebSocket；Socket.IO 是独立开放需求，不能混同。 [18](https://docs.usebruno.com/send-requests/grpc/grpc-streams)[3](https://www.usebruno.com/pricing)

### 6. OAuth2 系统浏览器与 PKCE

**已实现；官方操作文档确认。** 复用浏览器密码管理器和登录会话，经回调与 bruno:// 深链返回，校验 state，在本机完成 code+PKCE 交换。默认文档回调为 oauth.usebruno.com，也可自托管或 localhost；本地存储不代表授权过程无外部网络。 [19](https://docs.usebruno.com/auth/oauth2-2.0/system-browser)

### 7. 变量类型、作用域及环境继承

**已实现；继承有 v4.3.0 固定发布源码。** 全局、集合、环境、文件夹、请求、运行时作用域，另有提示值和进程环境语法。v4 起有字符串、数字、布尔、对象；extends 按祖先到子项合并，检测缺失父项与环。#292 仍开放，不能据此否认继承交付。 [20](https://docs.usebruno.com/variables/overview)[21](https://docs.usebruno.com/variables/environment-variables)[22](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-common/src/utils/environment-inheritance.ts)

### 8. 秘密值与共享文件分离

**已实现；本地秘密与 Ultimate 外部管理器分开。** 环境文件只存秘密名，值留本机加密存储且从集合导出排除；文档称优先系统加密，否则 AES256。Ultimate 外部管理器列 HashiCorp Vault、AWS、Azure、Google，按需获取并留内存；不能推断 1Password 已接入或普通变量已有初始值/本地值双层。 [23](https://docs.usebruno.com/secrets-management/secret-variables)[24](https://docs.usebruno.com/secrets-management/secret-managers/overview)

### 9. 响应示例与本机 Mock

**已实现；Mock 为 Beta，发行版权益未单独核实。** 保存多种请求与响应快照，以 Try 打开临时请求副本。Mock 从示例/OpenAPI 生成，支持方法/路径、条件、端口、延迟与日志；v4.3.0 有运行时和 Beta 开关。文档称退出即停、重启不自动启动，日志仅内存最近 500 条，不是审计服务。 [26](https://docs.usebruno.com/mock-servers/overview)[25](https://docs.usebruno.com/send-requests/res-data-cookies/response-examples)[27](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-electron/src/app/mock-server/mock-server.js)

### 10. 多工具和规范导入导出

**已实现；单集合与商业整批迁移有区别。** Bruno、Postman 集合、Insomnia JSON/YAML、OpenAPI JSON/YAML/URL、WSDL 导入；Bruno/Postman/OpenAPI 导出。Ultimate 整批 Postman 数据导入不同于单集合导入；不承诺脚本与认证无损迁移。 [28](https://docs.usebruno.com/get-started/import-export-data/import-collections)[29](https://docs.usebruno.com/get-started/import-export-data/export-collections)

## 与 Reva 当前实现逐项对照

检查了真实模型、传输、GUI 编排、变量、脚本、导入器、存储和同步代码；不把 README 未写直接当作缺失，也不把测试替身或软件自身 CI 当成用户功能。

| 维度 | Bruno | Reva 基线 | 改进方向 |
| --- | --- | --- | --- |
| 协议 | HTTP、GraphQL、原始 WebSocket、gRPC 四种调用；Socket.IO 独立提案。 | 已有 HTTP、多正文、GraphQL query/variables 编辑与 WebSocket 连接/消息；未发现 gRPC/Socket.IO 服务或编辑模型。 | 保留常用协议聚焦，gRPC 深度测试先配合 Bruno，避免引入重型运行时。 |
| 认证 | OAuth2 系统浏览器、授权码/PKCE；官方另列多类认证。 | Bearer、Basic、header/query API key；集合/文件夹继承、None 屏障、跨域剥离敏感认证；OAuth2 生命周期暂缓。S3 备份 SigV4 不是请求编辑器的 AWS 认证。 | 未来按场景独立做本机 OAuth2 生命周期，打开浏览器只是一步。 |
| 变量 | 多作用域、类型、父环境、提示值及本地秘密值。 | 集合后叠加活动环境，最后启用值优先；KV 是字符串，Environment 无父字段。pm.environment.set 与 pm.variables.set 共用本次脚本副本，不持久保存环境。 | 先做会话本地覆盖，减少个人值污染共享集合。 |
| 脚本与断言 | 多层脚本、bru API、Chai、声明式 Assert、安全/开发者模式。 | 已有 Otto 默认 2 秒限时脚本；有限 pm.test/pm.expect 支持状态、文本/JSON、耗时、头和控制台。拒绝异步测试，无 require/npm/网络/文件系统桥接，未见集合/文件夹脚本字段。 | 复用脚本内核，说明兼容子集，不引入 Node 追求脚本兼容。 |
| 运行器与 CI | 免费桌面集合运行；公共 CLI 有过滤、失败即停和报告。桌面数据驱动/并行 Pro 起，下载报告 Ultimate。 | GUI 单 tab 显式发送；HTTP 服务可并发调用，但没有用户集合批跑或 CLI 测试命令。仓库 CI 验证软件自身，不能代替用户 API 集合的 CI 执行。 | 提取共享执行管线，先提供有界顺序 CLI，不急做并行调度和数据矩阵。 |
| 导入导出 | Postman、Insomnia、OpenAPI YAML/JSON、WSDL 导入及 Postman/OpenAPI 导出；整批迁移另有商业权益。 | 已有原生集合/环境、Postman v2.0/v2.1、HAR、curl、Swagger2/OpenAPI3 JSON 与 URL 导入，含预览、兼容通知、确认及资源上限；原生集合 JSON 和单请求 curl 导出。Postman 脚本省略带通知，GraphQL/不支持认证阻止导入。未见 YAML/WSDL/Insomnia/Bruno 原生导入或 Postman 集合导出。 | Postman v2.1 单向导出连接 Bruno，明确脚本及变量语义损失，延续审查后确认。 |
| Mock 与示例 | 持久多响应示例及示例/OpenAPI 本机 Mock；后者 Beta。 | 有响应浏览、下载和请求历史；HistoryEntry 不存完整响应正文，没有持久响应示例或产品 Mock 服务。fake 是测试替身。 | 响应示例可后续小步增加；复杂 Mock 和共享服务先交给专业工具。 |
| 规范与结构校验 | OpenAPI 导入导出，脚本内 tv4/开发者模式 Ajv 校验 JSON Schema。 | OpenAPI 仅生成模板，省略响应定义、约束和外部引用；有限脚本断言，无 JSON Schema 桥接或运行时响应契约验证。JSON 可解析不等于符合 schema。 | 明确能力说明，完整契约覆盖、反例生成和模糊测试交给专业工具。 |
| 性能与负载 | 响应耗时、运行报告、付费并行；不能当作受控到达率、虚拟用户和百分位负载系统。 | HTTP 连接池复用；响应默认保留 10 MiB，超限继续读取但标记截断；有 DNS/连接/TLS/首字节阶段，无负载模型。README 后文 Windows 可执行文件实测约 15.03 MB，不能沿用开头约 5 MB 宣传或用于跨产品排名。 | 保留交互时间线；压力测试交专门工具，本次不声称资源/吞吐优劣。 |
| 协作与本地数据 | 文本项目可 Git 评审，YAML 默认、Bru 兼容；基础 Git UI 免费，高级付费；README 不提供集合云同步。 | 免账号，REVA_HOME JSON 目录与原子写入，每集合一个文件；已有可选 AES-GCM 备份及 WebDAV/S3/Gist 同步、冲突检测，不是多人实时协作。 | Git 保管审查后的可移植集合，分离本机偏好、历史与秘密覆盖，不建设内嵌 Git 平台。 |
| 秘密与商业边界 | 本地加密秘密与 Ultimate 外部管理器不同；高级 Git UI、数据驱动和桌面报告有付费边界。 | 输入遮罩仅显示行为，认证/变量可存本地 JSON 与原生导出；加密备份不等于本机凭据保险库。 | 先提供不落盘的会话覆盖并说明生命周期；系统密钥库/外部 Vault 单独设计。 |

**Reva 代码定位（相对于上述基线）：**

- **协议**：`internal/gui/graphql.go:initGraphQLRequest；internal/gui/websocket.go；internal/websocket/client.go；internal/service/request/request.go:Send；internal/model/model.go`。
- **认证**：`internal/auth/auth.go；internal/gui/auth.go；internal/model/tree.go；internal/service/request/options.go；internal/s3/sigv4.go；README.md`。
- **变量**：`internal/gui/env.go:VarsFor；internal/vars/vars.go:Lookup/Expand；internal/model/model.go:KV/Environment；internal/script/api.js；internal/script/script.go:RunPre`。
- **脚本与断言**：`internal/script/script.go:RunPre/RunTests/DefaultTimeout；internal/script/api.js；internal/gui/scripts.go`。
- **运行器与 CI**：`internal/gui/app.go:Send/declareSends；internal/service/service.go:RequestService；cmd/reva/main.go；.github/workflows/ci.yml`。
- **导入导出**：`internal/importer/importer.go；internal/importer/postman.go；internal/importer/openapi.go；internal/importer/har.go；internal/importer/curl.go；internal/gui/importer.go；internal/gui/collections.go:exportCollection；internal/gui/request.go`。
- **Mock 与示例**：`internal/model/model.go:Response/HistoryEntry；internal/gui/response.go；internal/service/fake/fake.go`。
- **规范与结构校验**：`internal/importer/openapi.go；internal/importer/diagnostics.go；internal/script/api.js；README.md`。
- **性能与负载**：`internal/service/request/request.go；internal/service/request/trace.go；internal/model/model.go:Response/Phase；README.md`。
- **协作与本地数据**：`internal/storage/storage.go；internal/service/store/collections.go；internal/syncing/sync.go；internal/cloudsync/cloudsync.go；internal/gui/sync.go`。
- **秘密与商业边界**：`README.md；internal/model/model.go:Auth/KV；internal/gui/auth.go；internal/syncing/sync.go:Seal/Capture`。

可点击入口：[模型](../../../internal/model/model.go)、[认证](../../../internal/auth/auth.go)、[变量合并](../../../internal/gui/env.go)、[脚本宿主](../../../internal/script/script.go)、[脚本 API](../../../internal/script/api.js)、[发送编排](../../../internal/gui/app.go)、[HTTP 传输](../../../internal/service/request/request.go)、[导入器](../../../internal/importer/importer.go)、[加密同步](../../../internal/syncing/sync.go)。

## 六个功能议题：需求与交付分开核实

功能列表只查询一次 GitHub 搜索接口：`repo:usebruno/bruno is:issue label:enhancement`，评论数降序前 100 条，返回总数 1410；从中选四个 open、两个 closed。以下标题、状态、标签、更新时间取自返回记录；关闭本身不是交付证据，开放本身也不是缺失证据。

### [#2650 Use system browser for OAuth authentification](https://github.com/usebruno/bruno/issues/2650)

- 状态：`closed`；最近更新：`2026-01-09T10:16:10Z`。
- 标签：`enhancement`, `onboarding`, `compatibility`, `short-term-goal`, `module-oauth2`, `P0`。
- 用户需求：复用系统浏览器密码管理器和登录会话，避免嵌入浏览器隔离。
- 交付判定：已核实交付；依据官方操作文档，不以关闭状态推断，未追溯首次合并 PR。
- 交付证据：https://docs.usebruno.com/auth/oauth2-2.0/system-browser。

### [#1235 Save Response Examples](https://github.com/usebruno/bruno/issues/1235)

- 状态：`closed`；最近更新：`2026-09-02T13:28:11Z`。
- 标签：`enhancement`, `critical`, `compatibility`, `module-qol`。
- 用户需求：把成功、错误及边界响应保存为可分享样本，不只保留最近结果。
- 交付判定：已核实交付；文档有保存步骤。v4.3.0 Try PR #9342 是后续重放能力，不是最初保存功能合并证明。
- 交付证据：https://docs.usebruno.com/send-requests/res-data-cookies/response-examples ; https://github.com/usebruno/bruno/releases/tag/v4.3.0。

### [#5887 Socket.IO support](https://github.com/usebruno/bruno/issues/5887)

- 状态：`open`；最近更新：`2026-11-09T11:51:03Z`。
- 标签：`enhancement`, `module-socket.IO`, `status-tracking`。
- 用户需求：原始 WebSocket 不能覆盖 Socket.IO 握手、事件与应用层语义，需要专门支持。
- 交付判定：开放提案；本次没有找到对应交付文档或合并证据。
- 交付证据：未核实交付；现有原始 WebSocket 不等同此功能。。

### [#1216 [Feature Request] variable initial and local value](https://github.com/usebruno/bruno/issues/1216)

- 状态：`open`；最近更新：`2026-10-07T14:30:55Z`。
- 标签：`enhancement`, `module-qol`, `module-variables`。
- 用户需求：分开共享初始值与个人本地值，避免同事配置互相覆盖或进入 Git。
- 交付判定：开放提案；秘密变量和运行时变量解决部分场景，未核实通用初始值/本地值双栏交付。
- 交付证据：https://docs.usebruno.com/secrets-management/secret-variables ; https://docs.usebruno.com/variables/overview。

### [#292 Nested Environments?](https://github.com/usebruno/bruno/issues/292)

- 状态：`open`；最近更新：`2026-09-29T08:25:31Z`。
- 标签：`enhancement`, `short-term-goal`, `module-environments`。
- 用户需求：共享环境基线，按开发/预发/角色继承，只维护差异。
- 交付判定：议题仍开放，但功能已核实：文档称 4.2.0 起有 extends，v4.3.0 源码有父链合并和环检测；不能据状态滞后认定缺失。
- 交付证据：https://docs.usebruno.com/variables/environment-variables ; https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-common/src/utils/environment-inheritance.ts。

### [#949 Import secrets from 1password](https://github.com/usebruno/bruno/issues/949)

- 状态：`open`；最近更新：`2026-11-07T05:36:33Z`。
- 标签：`enhancement`, `premium`, `module-secret-manager`。
- 用户需求：从已有 1Password 获取秘密以减少复制，也暴露系统能力与安全沙箱冲突。
- 交付判定：开放提案；Ultimate 已列四类外部管理器，但不含 1Password，未核实此需求交付。
- 交付证据：https://docs.usebruno.com/secrets-management/secret-managers/overview。

这里没有伪造“关闭未交付／重复”样本：所选两个关闭议题均有独立官方能力文档，未追溯最初合并 PR；#292 虽开放却已能在固定发布源码确认继承。其余开放项明确标为未核实完整交付，不把部分替代能力当作完成。

## 三个可实施的 Reva 改进

### 1. P0：会话本地覆盖，避免个人值进入共享集合

- 场景：两位开发者使用同一集合，分别填临时租户或 token，重放时保持自己的值。
- 缺口：VarsFor 只有集合和持久环境；脚本副本不是用户可管理的本地覆盖，改环境会改变 JSON 和导出。
- 最小 PR：环境 UI 增加明确标识的“仅本次会话”覆盖表，状态放特性 state；集合→环境→会话合并，支持空值、删除、禁用与来源说明，沿用占位模板。退出即丢弃，不改磁盘格式，不宣称秘密保险库。
- 依赖：复用 env.go:VarsFor 与 internal/vars，补 zh/en 及 headless 测试；无新依赖，不依赖父环境或账号。
- 验收：临时磁盘和 httptest：集合 A、环境 B、会话 C 时服务器收到 C；空字符串覆盖有效，删除回到 B，停用环境回到 A；重建 App 后 C 消失。发送、原生导出与 Capture 不把 C 写回集合/环境/历史模板，持久文件符合预期。

### 2. P1：共享执行管线与最小顺序集合 CLI

- 场景：提交前或内网 CI 跑一组回归请求，获取可靠退出码并定位请求或断言失败。
- 缺口：HTTP、认证继承、脚本均已存在，但编排绑定 GUI tab，无用户集合无窗口执行入口。
- 最小 PR：提取共享执行服务，复用认证作用域、变量、Options.Resolve 和前置/测试脚本；独立 CLI 显式选一个原生集合及环境，按树顺序深度优先执行，提供取消、超时、失败即停、JSON 汇总及退出码。第一版不做 GUI 批跑、并行、CSV、跨请求变量持久化。
- 依赖：需协调 app.go 调用同一管线；复用 model/tree、request、script、store，避免导入 GUI；先约定脚本错误是否阻断发送并保持两入口一致；不依赖会话覆盖。
- 验收：httptest/t.TempDir 验证嵌套认证、禁用变量、超时继承及断言在 GUI/CLI 产生相同请求和结果；失败或脚本超时退出非零，失败即停后后续端点零命中，取消停止剩余请求；JSON 含 ID/测试结果，默认不含认证值或正文。

### 3. P2：带兼容审查的 Postman v2.1 导出连接 Bruno

- 场景：日常在轻量 Reva 手调，发布前将可迁移 HTTP 集合交给 Bruno 的 runner、报告或后续 Mock 工作流。
- 缺口：能导入 Postman，但只导出原生集合和单请求 curl；Bruno 支持 Postman 集合导入，缺少交接出口。
- 最小 PR：仅导出明确支持的 HTTP 子集：文件夹、重复 KV、禁用行、正文、Basic/Bearer/API key 与继承；预览兼容清单，脚本 API 差异、会话值、文件引用须显式处理。无法等价表达时阻止或确认省略，默认保留凭据占位符，不自动执行外部工具。
- 依赖：复用 model/导入语义，新增纯 Go 导出包和保存 UI/i18n；先核对 Postman schema，不承诺 Bruno 无损翻译脚本；不依赖 Bruno 安装、账号或商业许可。
- 验收：离线 golden fixture 验证嵌套、空值、重复头及 auth 屏障；导出送回 Reva Postman 解析器，以 httptest 验证所声明子集等价；秘密会话哨兵不出现，不兼容脚本触发审查。另以固定 Bruno 版本人工验收导入并记录损失，不把第三方步骤加入仓库测试。

复杂数据矩阵、丰富报告、gRPC 与已有 Mock 工作流优先交给 Bruno；本地顺序执行器仅覆盖常见回归。负载模型、统计与完整规范反例测试交给专业测试工具，以审查后的导出连接；秘密管理器、Git 合并平台和 Node 包生态不应成为 Reva 的隐式前提。这些建议是独立设计方向，不涉及复制上游代码。

## 本地验证与限制

仅新增两份文档，无源代码、测试、图标或依赖变更，没有新增行为测试。检查时为 `go version go1.27.3 windows/amd64`，本进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`、`GOSUMDB=off`，测试没有使用互联网。项目要求 Go 1.26，此处如实记录实际工具链，不宣称是 1.26 复验。

四项仓库要求命令均退出 0，前三项输出为空，测试尾部原样如下：

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
ok  	github.com/runbrick/reva/internal/script	0.981s
?   	github.com/runbrick/reva/internal/service	[no test files]
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	1.835s
ok  	github.com/runbrick/reva/internal/service/store	1.478s
ok  	github.com/runbrick/reva/internal/storage	0.905s
ok  	github.com/runbrick/reva/internal/syncing	2.343s
ok  	github.com/runbrick/reva/internal/update	0.268s
ok  	github.com/runbrick/reva/internal/vars	1.387s
ok  	github.com/runbrick/reva/internal/websocket	0.671s
Exit: 0
```

另执行严格 UTF-8 JSON 解析、字段与数量断言、议题元数据对照、Markdown 相对链接检查及 `git diff --check`；不需要新行为测试。

1. 检索窗口为机器观测 2026-11-16T22:25:11+08:00 至 22:29:29+08:00，保留实际偏移而不伪写纽约时间。最新发布接口返回 v4.3.0（2026-10-22）；main/在线文档可能较新，仅固定 tag 证据标作发行源码确认。
2. 未安装运行 Bruno、未购买商业版；权益来自官方定价和文档，公共路径存在不证明二进制解锁。公共源码 MIT 属 OSI 开源，没有把受限源码可见授权标作 OSI，也不把整个商业产品当作 MIT。
3. GitHub 功能列表只查一次：repo:usebruno/bruno is:issue label:enhancement，评论降序前 100 项，总数 1410；取六项，四 open、二 closed。不是完整路线图，未访问受限 Jira，未专门确认关闭未交付/重复样本；首次合并 PR 未追溯，交付依据另有注明。
4. CLI 文档列 --csv-file-path、--json-file-path、--parallel，但公共 main 和 v4.3.0 run.js 未见对应参数声明；桌面付费范围明确，CLI 是否另有商业分发待核实。只确认公共 CLI 基本执行、过滤和报告，不把这些数据/并行参数算成 MIT 已交付能力。
5. Mock 文档与 v4.3.0 运行时/Beta 开关可核实，定价表未单列清发行版权益，保留未知。部分文档首次超时后重试成功；未做两产品启动、内存、包体或吞吐实测。
6. Reva 缺失判断限于本基线 model、cmd、service、gui、script、importer 等源码和关键词搜索，不推断其他分支或未来工作。TASK.md、本地 .claude/skills/building-with-viva/SKILL.md 均不存在；无 GUI 修改或新行为测试。
7. 四项仓库检查在 Windows/amd64、Go 1.27.3 下通过；本进程设置 GOFLAGS=-p=2、GOMAXPROCS=2、GOPROXY=off、GOSUMDB=off。项目要求 Go 1.26，未安装切换工具链，不宣称复验了 Go 1.26。

## 编号证据来源

1. [规范仓库、社区/商业声明与商标](https://github.com/usebruno/bruno/blob/main/readme.md)
2. [公共仓库 MIT 许可证](https://github.com/usebruno/bruno/blob/main/license.md)
3. [版本与功能权益表](https://www.usebruno.com/pricing)
4. [v4.3.0 发布记录，2026-10-22](https://github.com/usebruno/bruno/releases/tag/v4.3.0)
5. [Electron 主进程依赖与入口](https://github.com/usebruno/bruno/blob/main/packages/bruno-electron/package.json)
6. [React 渲染层与构建依赖](https://github.com/usebruno/bruno/blob/main/packages/bruno-app/package.json)
7. [OpenCollection YAML 默认与迁移](https://docs.usebruno.com/bru-lang/yaml-migration)
8. [免费与付费 Git UI 边界](https://docs.usebruno.com/git-integration/using-gui/intro)
9. [v4.3.0 公共 CLI 命令、报告与沙箱](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-cli/src/commands/run.js)
10. [CLI 参数文档，部分与公共源码不一致](https://docs.usebruno.com/bru-cli/run/options)
11. [CLI 报告及敏感字段省略](https://docs.usebruno.com/bru-cli/run/reports)
12. [声明式断言](https://docs.usebruno.com/testing/tests/assertions)
13. [内置库与 JSON Schema 校验库](https://docs.usebruno.com/testing/script/inbuilt-libraries)
14. [脚本作用域与请求链](https://docs.usebruno.com/testing/script/overview)
15. [安全与开发者模式](https://docs.usebruno.com/get-started/configure/javascript-sandbox)
16. [QuickJS 安全沙箱源码](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/quickjs/index.js)
17. [Node vm 开发者模式源码](https://github.com/usebruno/bruno/blob/main/packages/bruno-js/src/sandbox/node-vm/index.js)
18. [gRPC 四种调用、反射与 proto](https://docs.usebruno.com/send-requests/grpc/grpc-streams)
19. [OAuth2 系统浏览器与本地交换](https://docs.usebruno.com/auth/oauth2-2.0/system-browser)
20. [变量优先级、类型与存储](https://docs.usebruno.com/variables/overview)
21. [环境继承与兼容边界](https://docs.usebruno.com/variables/environment-variables)
22. [v4.3.0 环境继承与环检测](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-common/src/utils/environment-inheritance.ts)
23. [加密秘密值与导出排除](https://docs.usebruno.com/secrets-management/secret-variables)
24. [Ultimate 外部秘密管理器](https://docs.usebruno.com/secrets-management/secret-managers/overview)
25. [响应示例保存与临时重放](https://docs.usebruno.com/send-requests/res-data-cookies/response-examples)
26. [本地 Mock 服务 Beta](https://docs.usebruno.com/mock-servers/overview)
27. [v4.3.0 Mock 运行时与 Beta 开关](https://github.com/usebruno/bruno/blob/v4.3.0/packages/bruno-electron/src/app/mock-server/mock-server.js)
28. [集合导入与整批迁移限制](https://docs.usebruno.com/get-started/import-export-data/import-collections)
29. [集合导出格式](https://docs.usebruno.com/get-started/import-export-data/export-collections)
30. [桌面数据驱动、并行与报告的付费范围](https://docs.usebruno.com/testing/automate-test/data-driven-testing)
31. [Use system browser for OAuth authentification](https://github.com/usebruno/bruno/issues/2650)
32. [Save Response Examples](https://github.com/usebruno/bruno/issues/1235)
33. [Socket.IO support](https://github.com/usebruno/bruno/issues/5887)
34. [[Feature Request] variable initial and local value](https://github.com/usebruno/bruno/issues/1216)
35. [Nested Environments?](https://github.com/usebruno/bruno/issues/292)
36. [Import secrets from 1password](https://github.com/usebruno/bruno/issues/949)
