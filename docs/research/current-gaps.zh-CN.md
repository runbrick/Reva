# Reva 当前能力与调研缺口

代码基线：`481b84b`（PR #30 合并点）。下文先核对该提交中的实现，再单独记录本轮新增的工作台指南入口。这里的「已交付」指已进入基线代码，不代表逐个平台或安装包重验；「待做」是独立评审候选，不是交付承诺。

[综合研究](api-tools-comparison.zh-CN.md)、[实施计划](implementation-plan.zh-CN.md)、[首批交付报告](implementation-results.zh-CN.md)和 [EasyPostman 补充调研](api-tools/easy-postman.md)保留各自的历史基线。旧文档中的「仅 GUI」「无 SSE」「未做桌面 Runner」以及「建议增加集合到环境导入接续」不代表当前状态，不回写历史报告。

## 基线已交付

| 项目 | 当前能力及证据 | 不等同于 |
| --- | --- | --- |
| R1 共用执行语义 | [`execution.Plan.Run`](../../internal/execution/execution.go) 复用认证继承、请求选项、限时前置脚本、变量展开、传输与响应断言；[GUI 接线](../../internal/gui/app.go)沿用原始请求快照写历史。前置脚本失败不发送。 | 完整 Postman 脚本运行时、跨请求共享脚本值。 |
| R2 原生集合执行 | [`runner.Run`](../../internal/runner/runner.go) 对快照按集合树深度优先串行执行，可取消、可选遇错停止；不写集合、环境或历史。 | 任意步骤编排、选定文件夹单独运行、并行、数据迭代。 |
| R3 无窗口 CLI | [`cmd/reva-run`](../../cmd/reva-run/main.go) 运行已保存的原生集合，显式选择环境，默认不用环境；[`Report`](../../internal/runner/report.go) 输出版本化、最小化的 JSON 和稳定结果码。详见 [README](../../README.md)。 | 通用导入执行器、JUnit、包含请求和错误原文的调试报告。 |
| R4 发送前预览 | [`preview.Build`](../../internal/preview/preview.go) 及[预览界面](../../internal/gui/preview.go)解释变量来源、覆盖、未解析项和认证来源；默认遮盖变量值与凭据，不发送、不执行脚本、不读上传文件、不写盘。 | 脱敏保险库或脚本运行后的真实报文。 |
| R6 有界 SSE 检视 | 普通桌面「发送」识别最终响应的 `text/event-stream`；[`SendSSE`](../../internal/service/request/sse.go) 增量解析，[GUI mailbox](../../internal/gui/sse.go) 有界缓存并可断开。行/事件上限为 16/32 KiB，每标签保留最先 64 条且合计不超过 128 KiB；后续事件省略。详见 [SSE 说明](pr-easy-postman-sse.md)。 | 自动重连或断点续传。整次请求仍有超时；下载、Runner、CLI 仍走常规 HTTP 路径。 |
| 桌面 Runner | [集合根概览入口](../../internal/gui/collection_runner.go)在显式开始后对已保存数据取快照，复用 R2 与 [v1 脱敏报告](../../internal/runner/report.go)，显示进度并可取消；见[桌面验收](pr-easy-postman-desktop-runner.md)。 | EasyPostman 的任意顺序功能测试、CSV/JSON 迭代或压测。 |
| 设置中的可选上手指南 | [共享指南](../../internal/gui/onboarding.go)在设置中打开或重播，可跳过；基线已有[独立离线示例](../../internal/gui/onboarding_setup.go)和[常规导入入口](../../internal/gui/importer.go)的交接。 | 启动向导、联网示例或新的导入器。 |
| Postman 集合 → 独立环境 | [导入接续](../../internal/gui/import_environment_handoff.go)和[导入确认](../../internal/gui/importer.go)已支持新集合提交后可选接续独立环境：各自审查、各自确认，提示 URL 中缺失变量的名称；不自动激活、绑定或重写 URL。 | 原生 ApiPost/Apifox 项目包、两实体原子迁移或无损迁移。 |

## 本轮：从空白工作台发现同一指南

- [响应空态按钮](../../internal/gui/response.go)只在「接口」工作台的当前请求仍是未编辑的独立默认草稿、无执行结果，且指南未完成、本次会话未打开过时出现。启动不自动弹窗，不阻碍正常编辑；编辑后清空也不把该草稿当作从未使用。点击只打开[同一指南](../../internal/gui/onboarding.go)，不更改导航、当前请求或标签；门控见 [`showOnboardingEntry`](../../internal/gui/onboarding_setup.go)。
- 打开就会在本会话抑制工作台入口。「暂时跳过」或 Esc 只关闭，不落盘；未完成时冷启动可再出现。设置入口一直可用。只有点击「完成」才从最新设置保留其他偏好、写入现有 `OnboardingCompleted`，成功后关闭；失败可重试。完成后冷启动不再显示工作台入口，仍可从设置重播。无新 schema 或依赖。
- [离线练习研究](onboarding-practice.zh-CN.md)的边界不变：点击「离线预览示例」创建新的未保存 `GET https://example.invalid/hello` 草稿及现有只读预览，不覆盖已有草稿，不发送、不执行脚本、不读取上传文件，也不写设置、集合、环境或历史。占位 URL 不会产生模拟响应，另行点击「发送」才进入真实传输（可能尝试 DNS、连接或代理）。
- 「导入已有请求」只打开常规对话框。文件/粘贴的检查离线且不写应用数据；仅显式选择 Swagger URL 并点检查才取规范，不会发送导入的请求或运行脚本。兼容提示需确认；独立的最终确认才写集合或环境，单条 curl 则打开未保存草稿。打开、跳过、练习或取消不会触发备份/同步，也不撤销用户此前授权的操作。普通应用启动准备存储目录不属于指南写入。
- [回归](../../internal/gui/onboarding_entry_test.go)使用临时目录真实设置、禁止发送的替身与 Viva headless Driver。沿用桌面 Runner 的双次 resize 让分栏在 960 × 600 布局稳定后再做命中、点击和键盘验收；覆盖中英文及明暗主题，不是原生窗口的首帧或像素验收。

## 真正待做：优先级、边界与下次验收

| 优先级 / 缺口 | 当前差距和首版边界 | 进入合并前的最小验收 |
| --- | --- | --- |
| P1 · R5a 内存秘密覆盖 | [变量展开](../../internal/vars/vars.go)与[设置/模型](../../internal/model/model.go)没有秘密覆盖生命周期；预览遮盖和[加密快照](../../internal/syncing/sync.go)不是本地凭据保险库。先定义覆盖优先级及遮盖语义，只在明确的执行作用域有效，可清空，不引入持久化 vault。 | 用哨兵值证实覆盖生效且只存在于执行内存及获准请求；预览、日志、历史、原生导出、备份/同步及报告不含它；清除和重启后不再生效。 |
| P1 · R5b Cookie 会话 | [HTTP client 与响应 Cookie](../../internal/service/request/options.go)可展示 `Set-Cookie`，没有跨请求 jar。先独立评审默认关闭、单运行隔离的临时 jar；定义显式 `Cookie` 的优先级，不全局共享或落盘。 | 两个本地来源验证域、路径、Secure、重定向与显式 header 行为；关闭、清空后和下一次运行都不会沿用上次 cookie。 |
| P2 · R7a Hurl | [现有集合导出](../../internal/gui/collections.go)是原生格式，不是 Hurl。先转换可表达的 HTTP 请求与少量断言；不自动翻译任意脚本。 | 固定样本核对请求、重复/禁用行与断言；不支持项可见，认证参数化；先预览后明确保存，不启动 Hurl 或发送。 |
| P2 · R7b WireMock | [响应页](../../internal/gui/response.go)有查看和复制，不是持久化响应示例或 Mock 服务。先只导出用户选定响应的静态桩，不导出敏感值，不嵌入有状态匹配或服务器。 | 用固定样本验证方法、路径、状态码、可保留的响应头和正文；不支持或脱敏项在预览明示，确认才保存，不启动 WireMock。 |
| P2 · R7c k6 | [Runner](../../internal/runner/runner.go)是串行功能回归，不是负载模型。先导出可人工审查的单次 HTTP 流程；不运行脚本、不自动生成负载或阈值。 | 固定输出验证方法、URL、正文、禁用项与参数化认证；脚本和不支持的 body 明示，预览与保存分离；负载执行留给 k6。 |
| P2 · R8a CSV/JSON 迭代 | [`runner.Options`](../../internal/runner/runner.go)与 [CLI](../../cmd/reva-run/main.go)没有迭代输入；稳定执行与报告已有。先用有界文件、明确覆盖顺序和确定性逐行运行，不并行、不引入跨行变量传递。 | 行和字段错误可定位且不回显秘密；验证文件大小、行数与总请求上限，取消阻止后续迭代，各行隔离、不写环境，报告仍脱敏。 |
| P2 · R8b OAuth2 | [认证模型及校验](../../internal/auth/auth.go)仅支持现有继承、None、Bearer、Basic、API key；手填 Bearer 不代表 OAuth2 生命周期。会话边界确定后先做显式 `client_credentials`；不包办 PKCE 或浏览器授权。 | 本地 token 服务验证请求、到期、失败、取消及拒绝跨源转发秘密；client secret 和令牌不落盘，也不进入报告或导出。 |
| 后续 · 契约与 gRPC | [OpenAPI JSON 导入](../../internal/importer/openapi.go)只生成请求模板，[GraphQL 编辑](../../internal/gui/graphql.go)没有内省/模式浏览，[请求模型](../../internal/model/model.go)没有原生 gRPC。分开设计：先评估最小响应契约或失败重放交接，复杂生成交给 Schemathesis；gRPC 先评估显式本地 proto 的 unary 调用，不承诺反射或流式。 | 契约用固定规范和本地响应证明状态码、媒体类型漂移可定位，交接物可审查且无凭据；gRPC 若立项，用本地服务验证描述符选择、成功/状态错误、取消和大小上限。GraphQL 内省需单独明示联网并限制查询和结果大小。 |

## EasyPostman 仍值得跟踪的差异

- **迁移、变量与脚本（按样本立项）**：现有接续解决常见的 Postman 集合加环境路径，不覆盖原生 ApiPost、批量 cURL、Postman 保存响应或集合/文件夹脚本；[现有格式边界](../../README.md)继续适用。优先收集真实、去敏 fixture，先验证预检警告、两步确认和作用域无误，再决定是否做额外适配。当前是[单次变量展开](../../internal/vars/vars.go)和[有限脚本 API](../../internal/script/script.go)；递归变量若有需求，独立验收优先级、环/深度和脱敏，不借机开放脚本代发或任意网络。
- **更细的运行编排与企业传输（按场景）**：桌面与 CLI 已共享集合执行，但还没有任意顺序的功能测试配置、跨请求变量传递、Digest 或按主机的客户端证书选择。若新增选择范围/顺序，先验证显式保存范围、确定次序、取消及 CLI/GUI 报告一致性；企业传输则用本地端点独立验证认证和证书的主机边界，不与 R8b 混为一项。
- **重能力留在专用工具**：已有[加密快照及 WebDAV/S3/Gist](../../internal/gui/sync.go)，不是内置 Git 工作区或实时协作；先保持文件可审查，不代做 Git 合并或跨项目权限。保存响应虽可独立评估，但先定义显式保存、脱敏及删除；复杂 Mock、负载编排、多 worker 压测与契约模糊测试继续用专用工具。优先验证 R7 的手动交接，不内嵌运行时，也不扩展为 MCP 自动代发。
