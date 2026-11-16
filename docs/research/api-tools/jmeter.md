# Apache JMeter：对 Reva 的功能研究

> 文档研究 PR；Reva 基线 `72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；分支 `runbrick/research-jmeter`。
> 取证记录：`2026-11-16T22:34:32.485862+08:00`；上游源码固定 `ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c`。

## 结论与定位

JMeter 是面向可重复负载与性能分析的 Java 测试计划引擎，Reva 是面向本地交互调试的 Go/Viva 原生 API 客户端。值得借鉴的是运行模型、数据驱动和可解释的测试结果；多机压测与统计分析更适合通过明确格式边界对接 JMeter。

GUI 使用 Java/Swing 编辑测试树；JMeter 入口区分 GUI 与 CLI，SaveService 序列化 JMX，StandardJMeterEngine 配置和启动线程组，线程执行采样器及作用域内的处理器、断言、监听器。JTL 记录结果，HTML 报告离线聚合；RMI 分发完整计划到远端。当前官方稳定页是 5.6.3（Java 8+），所检源码是朝向 6.0.0 的开发快照（Java 17+），必须分别判断。

上游规范仓库为 [apache/jmeter](https://github.com/apache/jmeter)，ASF 官方 README 同时列出 GitHub 与 gitbox 源码入口 [1][2]。稳定能力主要按官方 5.6.3 文档判断；开发分支源码用于核实架构及特定 issue 的实际落地，不与稳定版混合。

## 许可证与社区边界

Apache JMeter 自身采用 OSI 批准的 Apache-2.0 开源许可证；本文所列核心 GUI、CLI、录制、分布式和报告均属 Apache 项目，无商业版解锁前提。第三方依赖、驱动、插件及商业托管服务须另行核验许可，不能把 Apache-2.0 统套到它们。

`LICENSE` 正文与 GitHub 仓库元数据均指向 `Apache-2.0`，并交叉核对 OSI 条目 [3][5]，不是仅“可查看源码”的商业许可证。`NOTICE` 及 README 的第三方声明应随重分发另行检查 [2][4]。不复制 JMeter 代码；所提建议采用格式互操作与现有 Go 能力。官方文档中的 Maven/Gradle/Jenkins 集成是第三方生态，不应写成官方内置团队云服务。

## 已实现的差异化能力（10 项）

| 能力 | 已实现范围与边界 | 一手证据 |
| --- | --- | --- |
| 多协议采样器 | 除 HTTP/HTTPS、SOAP/REST 外，内置 JDBC、JMS、LDAP、FTP、邮件、TCP、Java 等采样器；JDBC 驱动和 JMS 提供者实现需自行提供，不等于所有协议均开箱免配置。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#JDBC_Request) |
| 虚拟用户与负载模型 | 线程组配置并发用户、爬升、循环和持续时间；另有已实现但官方标为实验性的 Open Model Thread Group，按到达率描述升降负载。不要把开放的虚拟线程 PR #6855 算成已实现。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#Thread_Group) |
| 树状流程控制与事务 | Loop、If、While、ForEach、Module、Include 等控制器组合流程；Transaction Controller 汇总一组采样为事务，可配置是否计入定时器等时间。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#Transaction_Controller) |
| CSV 数据驱动 | 从本地 CSV 读取列变量，支持表头、带引号及换行值、文件共享模式、文件结束后的回收或停止线程；面向大量账号或测试数据。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#CSV_Data_Set_Config) |
| 响应提取与请求关联 | JSONPath、JMESPath、正则、CSS、XPath 等提取器将响应值写入变量，供后续请求使用；这是显式配置的关联，不是已交付的自动关联。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#JSON_Extractor) |
| JSR223 脚本扩展 | 脚本采样器及前后处理器、断言可使用 JSR223 语言；Groovy 支持编译缓存，适合重用计算逻辑。脚本在 JVM 中执行，不能类比为 Reva 的受限 JavaScript 接口或声称是安全沙箱。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#JSR223_Sampler) |
| 结构化断言 | 提供状态/内容、时长、大小、JSONPath/JMESPath 断言，以及 XML Schema Assertion；JSON Assertion 是路径和值验证，不是通用 JSON Schema/OpenAPI 契约验证。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#JSON_Assertion) |
| HTTP(S) 录制代理 | 通过浏览器显式配置代理，把 HTTP(S) 会话录成测试计划；HTTPS 需要信任录制证书。JMeter 不执行页面 JavaScript，也不测量浏览器渲染。 | [官方文档](https://jmeter.apache.org/usermanual/component_reference.html#HTTP(S)_Test_Script_Recorder) |
| 无界面执行与离线 HTML 分析 | jmeter -n -t plan.jmx -l results.jtl -e -o report 执行计划并生成报告；JTL 为结果文件，可生成 APDEX、吞吐、错误、分位数等报告。断言失败不应直接假设等同于 CLI 非零退出，CI 仍须按结果明确判定门槛。 | [官方文档](https://jmeter.apache.org/usermanual/get-started.html#non_gui) |
| 远程分布式执行 | 控制端通过 RMI 向多个服务端发送同一计划；每端运行完整线程数，不自动均分总并发。数据文件须另行分发，版本、证书、网络和结果回传资源需要协调。 | [官方文档](https://jmeter.apache.org/usermanual/remote-test.html) |

全部条目均能在官方稳定文档中找到；其中 Open Model Thread Group 官方标为实验性。完整 HTML 报告指标另见 [11]，远程执行约束见 [12]；一次请求的响应耗时与高并发负载性能是不同测量对象。

## 与当前 Reva 的逐项对照

下列 Reva 结论已检查实际代码，路径相对于仓库根目录；“未找到”限定于上述基线，不推断未来版本。

| 维度 | JMeter | 当前 Reva 与源码依据 | 可采取的方向 |
| --- | --- | --- | --- |
| 定位与架构 | Java/Swing 测试计划编辑器加独立引擎；测试树串起采样器、配置、控制器、前后处理器、断言和监听器。 | Go/Viva 原生桌面、服务接口与单请求发送流水线；并发安全的发送服务不是已有负载调度器。<br>依据：`cmd/reva/main.go:main；internal/service/service.go:RequestService；internal/gui/app.go:Send/run；internal/service/request/request.go:Service` | 保留轻量本地调试定位，把回归运行与性能压测明确分层。 |
| 协议与 HTTP/2 | 稳定版支持 HTTP(S)、JDBC、JMS 等；HTTP/2/HttpClient5 已合入开发分支，不能归入已发布 5.6.3。WebSocket/gRPC 不在所核验的内置采样器清单。 | HTTP(S)、独立 ws/wss 会话、GraphQL query/variables 编辑器已存在；HTTP 运输克隆 Go 默认 Transport，可能按服务端协商 HTTP/2，无显式版本选择，本次未实测；未找到 gRPC/JDBC/JMS 客户端。<br>依据：`internal/service/request/options.go:client；internal/websocket/client.go:Dial；internal/gui/graphql.go:initGraphQLBody/graphQLBodySection；internal/model/model.go:Request` | 不为协议数量追平 JMeter；数据库和消息系统性能测试交给专用工具，Reva 可突出已具备的 GraphQL/WebSocket 调试。 |
| 认证与会话 | Authorization Manager 支持 Basic、Digest、Kerberos（实现相关），Cookie Manager 维护会话；Bearer/API-key 可设请求头；不据此声称具备 OAuth2 授权生命周期。 | Bearer/Basic/API-key 及集合/文件夹继承、None 屏障和跨源重定向凭证剥离已实现；仅解析响应 Cookie，http.Client 未配置 Jar；OAuth2 生命周期未实现。<br>依据：`internal/model/model.go:Auth；internal/auth/auth.go:Resolve/ApplyHeaders；internal/service/request/options.go:client/responseCookies；internal/service/request/request.go:protectRedirects` | 回归链路应明确 Cookie 策略并保持现有认证优先级；不要以手动头替代完整 OAuth 流程的产品承诺。 |
| 变量与数据驱动 | ${name}、函数、线程变量和 CSV Data Set Config 支持多用户数据；运行时响应提取器能供后续采样器引用。 | {{name}} 展开、集合变量、环境覆盖、禁用行均已有；脚本 pm.environment.set 与 pm.variables.set 共用本次执行的变量副本，不持久化；RunTests 不返回变量变更，也不自动传到下一次手动发送；没有 CSV 迭代器。<br>依据：`internal/vars/vars.go:Lookup/Apply；internal/gui/env.go:VarsFor；internal/script/script.go:RunPre/RunTests；internal/script/api.js:variableAPI` | 先建立运行范围变量与隔离语义，再提供最小 CSV 数据驱动。 |
| 脚本与断言 | Groovy/JSR223 有采样器、前后处理、断言和编译缓存；可直接使用 JVM 生态。 | Otto JavaScript 已支持前置脚本、pm.test/pm.expect、响应 JSON/头/时长与日志；默认 2 秒超时，不支持异步测试；导入 Postman 脚本会告知省略，原生脚本保留。<br>依据：`internal/script/script.go:DefaultTimeout/RunPre/RunTests；internal/script/api.js；internal/gui/scripts.go:prepareSend/afterResponse；internal/importer/postman.go` | 复用已有脚本执行器实现批量测试，先定义错误及变量传播规则，避免另造一套脚本语言。 |
| 运行器、CI 与性能 | CLI 可复用 JMX、JTL 与 HTML；线程组、定时器、监听器和远程执行支撑性能场景；Maven/Gradle/Jenkins 集成为第三方。 | cmd/reva 直接打开 GUI；集合模型只存请求树，未找到集合运行调度、API 测试 CLI、分位数聚合或远程压测。现有 .github 工作流测试和打包应用，不能等同于用户 API 集合的 CI。<br>依据：`cmd/reva/main.go；internal/model/model.go:Collection；internal/gui/app.go:Send；.github/workflows/ci.yml` | 轻量串行回归值得做；高并发、分布式及统计报告交给 JMeter，不把单请求耗时图称为性能测试。 |
| 导入导出与迁移 | 原生 .jmx 为序列化测试树，JTL 是结果，不是 Postman 集合格式；录制器生成采样器。未核实官方内置 OpenAPI/Postman 无损转换器。 | 已审阅的导入器支持原生集合/环境、Postman v2、HAR、curl、Swagger 2/OpenAPI 3 JSON 和 URL 获取；导出集合为原生 JSON，未找到 JMX 导出；OpenAPI 导入只生成请求模板并提示丢弃项。<br>依据：`internal/importer/result.go:ParseWithOptions；internal/importer/openapi.go；internal/gui/importer.go:reviewImport/performImport；internal/gui/collections.go:exportCollection` | 只做有明确兼容边界的 JMX HTTP 子集出口，复用已有导入成果；不宣称脚本或计划可以双向无损转换。 |
| Mock 与录制 | 内置 HTTP Mirror Server 是回显调试服务器，可通过特殊请求头调整延迟、状态等；不是完整的有状态场景 Mock/契约服务。 | 未在服务接口、cmd 和 GUI 中找到供用户启动的 Mock 服务或录制代理；httptest 仅为开发测试，不算产品功能。<br>依据：`internal/service/service.go；cmd/reva/main.go；internal/gui/app.go；internal/service/request/request_test.go` | 有状态 Mock 更适合外接专用服务器；不把回显服务器当成完整 Mock 平台作为追赶目标。 |
| Schema 验证 | XML Schema Assertion 验证 XSD；JSON Assertion/JMESPath 检查路径和值，所核验文档不能证明原生 JSON Schema/OpenAPI 响应契约验证。 | JSON 语法编辑与脚本响应断言已存在；OpenAPI 导入生成示例，明确不验证 schema constraints，未找到响应契约验证器。<br>依据：`internal/gui/graphql.go:graphQLBodySection；internal/script/api.js:pm.response.json/expect；internal/importer/openapi.go:sample` | 不得把 JSON 可解析或导入 OpenAPI 等同于契约验证；未来单独评估契约工具集成。 |
| 协作、本地数据与商业边界 | Apache 发行版的 GUI/CLI/远程执行均可本地免账号使用；JMX、CSV、JTL 可用文件/Git 管理，但不是团队工作区、在线评审或协同权限服务。官方文档未设这些已列功能的商业版门槛。 | 本地明文 JSON 和 REVA_HOME 已有；另有 AES-GCM 加密备份及 WebDAV/S3/GitHub/Gitee 存储同步与冲突处理，不是实时多人协作或凭证保险库。<br>依据：`internal/storage/storage.go；internal/syncing/sync.go:Seal/Shared；internal/cloudsync/cloudsync.go；internal/gui/sync.go` | 保持免账号为默认；团队共享优先可审阅文件与明确冲突规则，不引入托管控制面。 |

认证和 Cookie 机制参见 [8] 的 HTTP Authorization Manager、HTTP Cookie Manager；Mock 比较对应 HTTP Mirror Server，schema 比较对应 JSON Assertion 与 XML Schema Assertion。架构和文件格式另以入口、引擎、线程组及 SaveService 源码交叉核对 [14]–[17]。

## 真实功能需求：GitHub issues

样本来自一次 `issues?state=all&per_page=100&sort=updated&direction=desc` 列表，随后读取直接页面；排除 PR、依赖更新与例行缺陷，得到 4 条 open、2 条 closed。下表精确保留英文原标题、标签与 UTC 更新时间；标签如 `to-triage` 不代表项目已经承诺实现。历史 Bugzilla 编号与 GitHub 编号不同，本文用迁移后的 GitHub 编号 [21]。

### #6467 — jdbc sampler to run script (multiple statements)

- 记录：[https://github.com/apache/jmeter/issues/6467](https://github.com/apache/jmeter/issues/6467)；状态 `open`；标签 `enhancement`、`to-triage`；更新 `2026-11-15T14:03:03Z`。
- 揭示的需求：批量建表、准备数据和依赖插入需要多条 SQL 的统一事务/回滚，不愿手工拆成许多采样器。
- 交付判定：**开放提案；本次未核验交付**。
- 证据范围：直接 issue 的用例/建议与开放状态；未见本次可核验的已合并 PR、稳定版发行或官方已实现文档，因此不能加入已实现能力清单。

### #6468 — JMeter HTML Report - Provision for Custom Decimal Percentiles Inc

- 记录：[https://github.com/apache/jmeter/issues/6468](https://github.com/apache/jmeter/issues/6468)；状态 `open`；标签 `enhancement`、`to-triage`；更新 `2026-11-15T13:12:42Z`。
- 揭示的需求：尾延迟 SLO 希望查看 p99.95、p99.99 等小数分位数，现有报告自定义粒度不能满足提问者需求。
- 交付判定：**开放提案；本次未核验交付**。
- 证据范围：直接 issue 的用例/建议与开放状态；未见本次可核验的已合并 PR、稳定版发行或官方已实现文档，因此不能加入已实现能力清单。

### #5083 — Let's have Plugins Manager added in JMeter by default

- 记录：[https://github.com/apache/jmeter/issues/5083](https://github.com/apache/jmeter/issues/5083)；状态 `open`；标签 `enhancement`、`os: All`、`P2`；更新 `2026-11-09T15:07:26Z`。
- 揭示的需求：安装新版本后反复手动复制插件管理器 JAR；希望发现与安装扩展成为内置流程。
- 交付判定：**开放提案；本次未核验交付**。
- 证据范围：直接 issue 的用例/建议与开放状态；未见本次可核验的已合并 PR、稳定版发行或官方已实现文档，因此不能加入已实现能力清单。

### #6700 — Response Assertion: print each pattern in output

- 记录：[https://github.com/apache/jmeter/issues/6700](https://github.com/apache/jmeter/issues/6700)；状态 `open`；标签 `enhancement`、`to-triage`；更新 `2026-11-13T12:06:53Z`。
- 揭示的需求：只拿到 JTL 的报告阅读者需要知道每个断言模式及其通过/失败，避免脱离 JMX 后结果不可解释。
- 交付判定：**开放提案；本次未核验交付**。
- 证据范围：直接 issue 的用例/建议与开放状态；未见本次可核验的已合并 PR、稳定版发行或官方已实现文档，因此不能加入已实现能力清单。

### #4023 — Support HTTP/2 protocol

- 记录：[https://github.com/apache/jmeter/issues/4023](https://github.com/apache/jmeter/issues/4023)；状态 `closed`；标签 `P1`、`enhancement`、`os: All`；更新 `2026-10-29T17:14:45Z`。
- 揭示的需求：服务端已支持 HTTP/2，用户希望负载测试实际走该协议，而非仍报告 HTTP/1.1。
- 交付判定：**已核验交付到开发分支；尚无稳定版发布证据**。
- 交付证据：PR [#6742](https://github.com/apache/jmeter/pull/6742) 的 API 元数据 `merged_at=2026-10-29T17:14:42Z`，PR 正文明确列出修复 #5081/#4023；固定源码有 `HTTPHC5Impl`，开发版变更说明同时列出这两个 issue [18]–[20]。这是同一交付的两个需求视角，不是两个独立发布；[6][7] 仍是 5.6.3，未核验到包含该功能的稳定发行。

### #5081 — HTTP Request: Migrate to HTTPClient 5 APIs

- 记录：[https://github.com/apache/jmeter/issues/5081](https://github.com/apache/jmeter/issues/5081)；状态 `closed`；标签 `enhancement`、`os: All`、`P2`；更新 `2026-10-29T17:37:32Z`。
- 揭示的需求：希望 HTTP 客户端迁移到 HttpComponents 5，为协议演进及 HTTP/2 支持提供基础。
- 交付判定：**已核验交付到开发分支；尚无稳定版发布证据**。
- 交付证据：PR [#6742](https://github.com/apache/jmeter/pull/6742) 的 API 元数据 `merged_at=2026-10-29T17:14:42Z`，PR 正文明确列出修复 #5081/#4023；固定源码有 `HTTPHC5Impl`，开发版变更说明同时列出这两个 issue [18]–[20]。这是同一交付的两个需求视角，不是两个独立发布；[6][7] 仍是 5.6.3，未核验到包含该功能的稳定发行。

“关闭未交付/重复”也是独立类别，不能由 closed 自动推导 delivered；本样本的两个关闭项有具体合并及源码证据，均归为“开发分支已核验交付”，没有为了凑类别虚构重复关闭案例。开放的自动关联 PR #499 和虚拟线程 PR #6855 仅作为边界反证 [22][23]，不计入以上 6 条 issue，也不把它们写成现成功能。

## 给 Reva 的三个最小改进 PR

当前证据没有需要强行列为 P0 的缺陷；优先级反映与本地免账号调试方向的关联。以下均是后续建议，不属于本次文档提交的实现范围。

### P1：集合串行回归与独立运行变量

- 用户场景：本地开发者一次验证登录、查询、修改三个请求，并在失败时定位是哪条请求或哪项断言。
- 已有缺口：已有脚本和集合树，但发送只绑定单标签页；脚本变量副本不跨请求，缺少运行汇总与取消后的明确状态。
- 最小 PR：新增可测试的串行运行服务和最小 Viva 运行结果列表；拍平所选文件夹内 HTTP 请求，保持顺序，运行范围合并变量并复用认证解析、Options.Resolve、RunPre、Send、RunTests；为当前不返回变量的 RunTests 增加运行器适配接口，显式返回变量变更，仅在本次运行传给后续请求；UI 用 ui.Effect。默认单轮、无并发、不自动 Cookie 持久化，预检并拒绝混入 WebSocket 项；定义前置脚本异常停止和断言失败后继续的规则。
- 依赖：现有 model.Collection/Request、auth.Resolve、vars 与 script API；需要单独获批修改服务/GUI 所有权；不是本次文档 PR 的实现；先明确运行变量、失败、取消及 Cookie 策略；不引入账号或第三方依赖。
- 有意义的验收：仅用 httptest：登录响应脚本提取 token，下一条请求收到新 token；第二项断言失败仍产生具名失败结果，后续请求按规则执行；取消阻止尚未发出的请求；原集合、环境和脚本明文逐字不变，下一次运行不泄漏 token。

### P2：受限 CSV 数据驱动回归

- 用户场景：开发者用一份本地账号/边界值表重复运行相同请求流程，不反复修改环境变量。
- 已有缺口：集合/环境变量和单次脚本已存在，但没有数据行绑定、行号结果或 CSV 文件结束语义。
- 最小 PR：在串行运行器加可选 CSV 输入，使用 Go encoding/csv；首行为变量名，限制文件大小/行数/字段数，预览冲突并定义数据列覆盖运行初始变量；每行独立建立运行变量，不跨行保留脚本写入；只读本地显式选择文件，单并发，遇不合法 CSV 在发请求前整体拒绝，不加入随机分配或远程数据。
- 依赖：推荐 1 的运行器与结果模型；现有变量覆盖规则和 Viva 文件选择；明确数据列优先级、行隔离与凭证显示策略；无新依赖。
- 有意义的验收：临时 CSV 含中文、引号内逗号与换行，httptest 确认各行值准确且结果带行号；上一行脚本写入不泄漏到下一行；重复列名、超限和坏引号输入不发送任何请求；取消不会启动后续行。

### P2：向 JMeter 导出严格受限的 HTTP 计划

- 用户场景：用户在 Reva 调通几个接口后，需要用 JMeter 做较长时间或多机压测，继续使用自己的本地工具链。
- 已有缺口：Reva 没有压测调度，也不能直接导出 .jmx；手工复制 URL、重复参数、请求头和认证容易丢失语义。
- 最小 PR：新增纯 Go、无执行行为的 JMX 导出器及预览确认入口；首版只接收显式选择的 HTTP 请求、无脚本、无变量、无认证、无文件引用，支持完整 URL、重复查询行/头和原始文本或 JSON 正文；生成固定单用户单轮的最小计划供用户在 JMeter 中调整。任何不支持的 auth、继承、脚本、模板、文件、会话或传输选项均阻断并说明，不静默删除；在 JMeter 一侧配置真实负载和凭证，不在 Reva 嵌入 JVM。
- 依赖：先锁定 Apache JMeter 5.6.3 的 JMX 支持子集及样例；现有集合选择、导出文件与导入兼容性提示模式；首版不依赖推荐 1/2，不安装 JMeter 或引入 Go 依赖；人工可选验收使用外部 JMeter。
- 有意义的验收：纯 Go 单测验证 XML、URL 编码、重复头/参数及 UTF-8 正文不丢失；脚本、变量、认证、文件或非默认请求选项均在落盘前拒绝；额外由维护者在 JMeter 5.6.3 打开产物，仅向本机 httptest 服务跑一轮并比较 method/path/query/headers/body，不访问互联网。

分工建议：Reva 负责调通 API、可读回归和数据预览；JMeter 负责负载曲线、多机执行、监听器与分位数报告。不要在 Viva 应用里嵌入 Java/Swing 或重造分布式压测器；也不因 JMeter 有录制器就给 Reva 引入证书代理这一整套维护面。

## 限制与取证范围

- 检索于本机 2026-11-16 完成，保留工具返回的 +08:00 时间偏移；不根据会话预设时区改写为未观测的本地时钟。retrieved_at 为最后一次取证后的记录，首批请求时间为 2026-11-16T22:27:45.120822+08:00。
- 未安装或运行 JMeter，也未做真实网络压测、协议互通或吞吐基准；已实现的判断来自官方稳定文档和源码，不给出性能优劣排名。
- 官方站点下载页和 changes.html 仍为 5.6.3，开发分支注明 6.0.0/Java 17；不以 master README 的 Java 17 要求覆盖稳定版 Java 8+ 要求，不把合并 PR 当成稳定版发布。
- GitHub 问题列表只获取一次：state=all、按 updated 降序、每页 100；其中包含 PR，挑出的 6 条均为 issue，4 open/2 closed，不是全库需求频次或投票统计；两个 closed 共用同一项合并交付。
- 按 API 快照记录标题、状态、标签、更新时间，并查看直接 issue/PR 页面；开放项本次未发现可核验交付不等于断言全历史没有任何局部实现；后续状态可能变化。
- 上游源码固定为 ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c；官方网站链接会滚动变化。未核验第三方插件或商业托管服务的全部能力和许可证，均不计入 Apache 发行功能。
- Reva 无某功能的表述来自当前入口、模型、服务、GUI、导入器和脚本源码的联合检查，限定于基线；没有仅因 README 未提及而判定缺失。HTTP/2 的实际协商行为未实测。
- 本次只改两份研究文档，无新增行为测试、图标、依赖、源码或配置；TASK.md 和本地 Viva 技能文件确实缺失，按任务说明不涉及 GUI 编码。

## 本次文档 PR 验证

没有新增行为测试。检查仅运行仓库既有测试，不下载研究对象依赖；不将互联网用于测试。为降低并行资源占用，本进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`。

四项仓库要求的检查均退出 0；前三项无输出（`gofmt -l .` 为空）。测试尾部如下，完整进程日志保存在本机临时研究目录，不作为提交文件。

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
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	0.959s
ok  	github.com/runbrick/reva/internal/service/store	0.521s
ok  	github.com/runbrick/reva/internal/storage	1.710s
ok  	github.com/runbrick/reva/internal/syncing	2.250s
ok  	github.com/runbrick/reva/internal/update	1.226s
ok  	github.com/runbrick/reva/internal/vars	0.795s
ok  	github.com/runbrick/reva/internal/websocket	0.397s
Exit: 0
```

JSON 用 Python 标准库验证必需字段、数组结构、枚举、issue 计数与状态；Markdown/JSON 通过 `git diff --check`。只提交本目录的 `jmeter.md`、`jmeter.json`，无源码、测试、依赖或图标变更。提交哈希不写回自身文档，以最终 worker_done 报告为准。

## 编号证据来源

1. [Apache JMeter 规范仓库](https://github.com/apache/jmeter)
2. [项目定位、协议、扩展、运行和依赖说明](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/README.md)
3. [Apache-2.0 许可证正文](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/LICENSE)
4. [版权与声明](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/NOTICE)
5. [OSI 的 Apache License 2.0 条目](https://opensource.org/license/apache-2-0)
6. [官方稳定版下载页：5.6.3](https://jmeter.apache.org/download_jmeter.cgi)
7. [稳定版 5.6.3 变更说明](https://jmeter.apache.org/changes.html)
8. [稳定文档：采样器、控制器、CSV、断言、提取器、录制和回显](https://jmeter.apache.org/usermanual/component_reference.html)
9. [变量与函数参考](https://jmeter.apache.org/usermanual/functions.html)
10. [无界面执行、结果和启动要求](https://jmeter.apache.org/usermanual/get-started.html#non_gui)
11. [HTML 报告与统计边界](https://jmeter.apache.org/usermanual/generating-dashboard.html)
12. [远程执行和数据分发约束](https://jmeter.apache.org/usermanual/remote-test.html)
13. [脚本和性能测试实践](https://jmeter.apache.org/usermanual/best-practices.html)
14. [GUI/CLI 入口源码](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/src/core/src/main/java/org/apache/jmeter/JMeter.java)
15. [测试引擎源码](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/src/core/src/main/java/org/apache/jmeter/engine/StandardJMeterEngine.java)
16. [线程组调度源码](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/src/core/src/main/java/org/apache/jmeter/threads/ThreadGroup.java)
17. [JMX 测试树序列化源码](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/src/core/src/main/java/org/apache/jmeter/save/SaveService.java)
18. [开发分支 6.0.0 变更说明，非发布声明](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/xdocs/changes.xml)
19. [已合并的 HttpClient5/HTTP2 实现及测试](https://github.com/apache/jmeter/pull/6742)
20. [开发分支 HTTP 客户端 5 实现](https://raw.githubusercontent.com/apache/jmeter/ba2e9ba15a73ddac44e8ef28e3dbdf4d155b5f3c/src/protocol/http/src/main/java/org/apache/jmeter/protocol/http/sampler/HTTPHC5Impl.java)
21. [官方问题追踪入口与历史 Bugzilla 迁移范围](https://jmeter.apache.org/issues.html)
22. [仍开放的自动关联提案，用于避免误判手动提取为自动关联](https://github.com/apache/jmeter/pull/499)
23. [仍开放的虚拟线程提案，用于避免把提案算作现有执行架构](https://github.com/apache/jmeter/pull/6855)
24. [jdbc sampler to run script (multiple statements)](https://github.com/apache/jmeter/issues/6467)
25. [JMeter HTML Report - Provision for Custom Decimal Percentiles Inc](https://github.com/apache/jmeter/issues/6468)
26. [Let's have Plugins Manager added in JMeter by default](https://github.com/apache/jmeter/issues/5083)
27. [Response Assertion: print each pattern in output](https://github.com/apache/jmeter/issues/6700)
28. [Support HTTP/2 protocol](https://github.com/apache/jmeter/issues/4023)
29. [HTTP Request: Migrate to HTTPClient 5 APIs](https://github.com/apache/jmeter/issues/5081)
