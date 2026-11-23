# Reva 功能调研与候选清单

基于 `99c3363` 的代码与 README，外部资料查阅于 2026-11-23。优先级按日常调试频率、能否复用现有功能、误操作风险和维护成本排序。各项均给出首版范围和验收场景，供独立立项时使用。[Reva README](../../README.md) · [早期调研与缺口](current-gaps.zh-CN.md)

## 已有能力

工作台已有集合切换、跨集合搜索、命令面板、标签草稿、关闭标签恢复、发送前预览与复制 cURL。响应区已有正文搜索、时间线、JSON 基线对比、字段断言与 JSON 字段捕获、单个响应示例和 WireMock 导出，也支持 HTTP、GraphQL 根字段浏览、WebSocket 和有界 SSE 检视。[标签](../../internal/gui/tabs.go) · [全局搜索](../../internal/gui/search.go) · [响应区](../../internal/gui/response.go) · [响应示例](response-examples.md) · [GraphQL](graphql-schema.md)

桌面 Runner 与 CLI 已支持文件夹范围、CSV/JSON 数据迭代及临时 Cookie 会话，CLI 可导出 JUnit。集合可生成脱敏的 Reva、Postman、Hurl、k6 分享模板。迁移入口已有导入预检，支持 OpenAPI/Swagger JSON、Postman、HAR、单条 cURL 和原生格式。[Runner](runner-workflows.md) · [分享模板](share-exports.md) · [导入支持](../../README.md#import-and-migration)

## P1 · 优先改善日常调试

| 候选功能 | 首版范围与用户收益 | 验收场景 |
| --- | --- | --- |
| **JSON 响应筛选** | 在现有文本搜索旁提供 JSONPath 结果视图，让用户从大对象或数组中找出一组值。首版支持根、子字段、数组索引和通配符，界面标明支持的语法与查询上限。[Postman 响应筛选][postman-response] · [RFC 9535][jsonpath] | 从订单列表一次选出所有订单 ID，分别提示语法错误、无匹配和输入过大。结果可复制，原始正文保持完整。 |
| **重定向链诊断** | 当前时间线只保留最后一跳的阶段。增加本次已发生的跳转列表，显示方法、状态码、脱敏目标及用时，并指出认证何时被移除。[Reva 时间线](../../internal/service/request/trace.go) · [Postman Console][postman-console] | 在两个本地测试服务间验证跳转、方法变化和跨源认证移除。未跟随的跳转标注原因，复制诊断摘要时省略查询值与凭据。 |
| **Runner 配置预设** | 保存一套常用的集合范围、环境、遇错停止开关与 Cookie 选项。打开预设就能复用配置，数据文件每次由用户选择，点击运行前显示实际范围。[Reva Runner](runner-workflows.md) · [Postman Runner][postman-runner] | 重启后从预设运行同一组冒烟测试。文件夹或环境被删除时提示重新选择，报告仍沿用现有脱敏格式。 |
| **会话级秘密覆盖** | 允许用户在选定环境的执行会话里临时填入令牌和密钥。先明确它与集合、环境、数据行、捕获和脚本变量的优先顺序，发送前预览显示来源并遮盖值。[Reva 变量预览](../../internal/preview/preview.go) · [Postman Vault][postman-vault] · [JetBrains 私有环境变量][idea-variables] | 用唯一测试值验证只有选定作用域的显式发送能使用它。保存、导出、同步、历史、报告和脚本日志里都找不到测试值，清空或重启后要求重新输入。 |
| **跨重启恢复已保存的标签** | 恢复已保存请求的标签顺序、选中项和集合焦点。首版只记录已保存请求的标识和界面顺序，减少每天重新打开常用接口的步骤。[Reva 标签状态](../../internal/gui/tabs.go) | 重新打开应用后按原顺序恢复请求。已删除的请求被跳过，加载过程零发送，也不会恢复旧连接。 |
| **双环境对比向导** | 在已有 JSON 基线对比上加「用环境 A 发送」「用环境 B 发送」两个明确步骤。每步先显示所选环境和脱敏目标，结果并列显示状态码、耗时和字段差异。[Reva 响应工具](../../internal/gui/response_tools.go) | 同一已保存请求在两个本地环境产生可定位的差异。完成后保持原环境选择，两个环境均需单独确认发送。 |

## P2 · 扩大回归、迁移和分享能力

| 候选功能 | 首版范围与用户收益 | 验收场景 |
| --- | --- | --- |
| **多组具名响应示例** | 每个已保存请求可保留多组示例，例如成功、校验失败和空列表，单独命名、预览、删除或导出为 WireMock 映射。沿用正文显式保存。[现有响应示例](response-examples.md) · [WireMock 调研](api-tools/wiremock.md) | 先前保存的单示例仍能打开。保存第二组后可独立选择其中一组删除或导出，空白正文可作为有效样例保存。 |
| **响应头和 Cookie 字段捕获** | 在已有 JSON 字段捕获上支持从受选响应头取单值，例如 `Location`。Cookie 捕获进入隔离的内存执行作用域，可标为敏感值，沿用 Runner 的迭代隔离。[现有捕获规则](../../README.md#response-tools) · [VS Code REST Client 请求变量][vscode-http] | 后续步骤能使用提取到的 `Location`，重复头值的取法可预测。缺失字段给出可定位的信息，敏感值不会进入诊断输出，下一迭代从空捕获状态开始。 |
| **生成 Go、Python 和 JavaScript 请求代码** | 在现有复制 cURL 旁为单条 HTTP 请求增加可预览代码片段。先支持无正文、文本和 JSON，保留重复请求头的语义，认证默认使用可替换的变量。[VS Code REST Client 代码生成][vscode-http] | 含 Unicode、引号及换行的输入生成语法有效的片段。预览明确列出无法表达的选项，复制结果经过敏感值测试。 |
| **`.http` 文件导入** | 支持 `###` 分隔的多条请求，保留方法、URL、变量占位符、重复头和内联文本正文。复用现有检查与确认，对脚本、文件引用及动态变量报告兼容性提示。[IntelliJ HTTP Client][idea-http] · [VS Code REST Client][vscode-http] | 把含三条请求的项目文件预览成三条草稿或集合条目，端点和原顺序正确。审查过程只读取用户选中的文件，取消后无数据写入。 |
| **OpenAPI YAML 导入** | 让 `.yaml` 和 `.yml` 进入现有 OpenAPI 预检，继续复用现有操作、参数和正文映射。需求评审时核对受限解析实现与项目依赖约束。[Apifox OpenAPI 导入][apifox-import] · [Reva OpenAPI 映射](../../internal/importer/openapi.go) | 等价 JSON 与 YAML 产生相同条目和警告。重复键、过深输入、循环展开和外部引用明确报错，预检离线完成。 |
| **OpenAPI 增量更新预览** | 为已导入操作记录可匹配的来源标识。用户选择新规范后，按稳定 `operationId` 或方法与路径预览新增、修改和移除。仅应用主动勾选的操作，遇到本地编辑冲突时提示逐项处理。[Apifox 导入设置][apifox-options] · [OpenAPI-diff][openapi-diff] | 重新导入时只新增选中的路径。操作改名、移除及本地手改都有单独提示，原有脚本和认证保留，取消后集合字节不变。 |
| **OpenAPI 响应契约检查** | 让用户为请求选择规范中的一个操作。首版核对状态码、媒体类型、JSON 必填项与基本类型，并把失败位置显示在响应面板或 Runner 摘要中。[JSON Schema 对象规则][json-schema] · [Schemathesis 调研](api-tools/schemathesis.md) | 固定规范与本地服务能分别触发错误状态码、媒体类型和字段类型。区分「未配置规则」「通过」「检查失败」，报告只含最小诊断信息。 |
| **Git 友好的集合目录导出** | 把集合显式导出为按请求拆分、有稳定文件名和顺序清单的目录，便于代码审查。默认使用脱敏模板，提供 Git 外部操作所需的文档。[Bruno Git][bruno-git] · [Insomnia Git Sync][insomnia-git] · [Reva 分享模板](share-exports.md) | 同一集合连续导出两次的内容一致。只改一个请求时差异集中在该文件，重排可从顺序清单看出，重名请求不会覆盖彼此。 |

## P3 · 有实际使用场景后立项

| 候选功能 | 首版范围与用户收益 | 验收场景 |
| --- | --- | --- |
| **按主机配置客户端证书** | 面向内网和网关 mTLS。用户明确选择客户端证书与私钥，按目标主机和端口匹配。可单独评估用户配置自有 CA，继续校验服务器身份。[EasyPostman 证书调研](api-tools/easy-postman.md) · [Reva TLS 选项](../../internal/service/request/options.go) | 本地 mTLS 服务验证正确主机成功、错误主机收不到客户端证书，跨主机跳转重新匹配。集合分享产物中无私钥或证书文件路径。 |
| **OAuth2 客户端凭证** | 在会话级秘密覆盖明确后，先服务机器到机器接口。显式获取令牌，展示有效期，并给用户重新获取与清除入口。桌面用户授权流程可后续按真实身份提供方需求独立评估系统浏览器与 PKCE。[Postman OAuth2][postman-oauth] · [RFC 6749 客户端凭证][oauth-client] · [RFC 8252 原生应用][oauth-native] | 本地令牌服务覆盖有效、到期、失败和取消。令牌按已选配置隔离，刷新状态可见，日志与导出不含令牌或客户端密钥。 |
| **SSE 事件筛选与详细查看** | 在有界事件列表上按事件名和正文筛选，点击一条查看完整的已保留内容或复制。显示省略计数，让使用者知道搜索覆盖的事件范围。[当前 SSE 边界](pr-easy-postman-sse.md) · [HTML SSE 规范][sse] | 长连接下筛选保持响应，切换筛选条件不会改变连接。超出保留量时仍可识别省略数量，断开后可检查本次已保留事件。 |
| **GraphQL 类型详情和字段插入** | 在根字段浏览之后补参数、必填项及返回对象的字段详情，用户可把所选字段插入当前查询草稿。模式仍由用户显式加载，详情沿用端点和大小边界。[当前 GraphQL 浏览](graphql-schema.md) | 从含必填参数的查询根字段生成可编辑片段，参数显示占位提示。切换端点清空旧类型详情，编辑本身不触发发送。 |
| **gRPC 一元调用** | 有内部服务样本时先验证显式选择本地服务描述符，选择服务和方法，以 JSON 编辑输入并显示状态、元数据及耗时。流式调用和反射依实际需求再定。[gRPC 核心概念][grpc] · [Yaak 的 gRPC 请求][yaak-grpc] | 本地服务覆盖成功、状态错误、取消、消息大小限制和无效描述符。支持范围与普通 HTTP Runner 的范围在界面上可辨识。 |

## 建议交付顺序

1. 先做 **JSON 响应筛选、重定向链诊断、Runner 配置预设**。验收围绕找字段、解释请求为什么跳转、重复跑同一套测试三个常见操作。
2. 接着做 **会话级秘密覆盖**，用它为敏感捕获和 OAuth2 提供统一的内存作用域。
3. 以团队提交的迁移样本选择下一入口。以规范为主的团队先走 **OpenAPI YAML → 增量更新预览 → 响应契约检查**。以源码仓库为主的团队可先做 **`.http` 导入 → Git 友好目录导出**。

[postman-response]: https://learning.postman.com/docs/use/send-requests/response-data/responses.md
[postman-console]: https://learning.postman.com/docs/use/send-requests/response-data/troubleshooting-api-requests.md
[postman-runner]: https://learning.postman.com/docs/tests-and-scripts/running-collections/intro-to-collection-runs.md
[postman-vault]: https://learning.postman.com/docs/use/postman-vault/postman-vault-secrets.md
[postman-oauth]: https://learning.postman.com/docs/use/send-requests/authorization/oauth-20.md
[jsonpath]: https://datatracker.ietf.org/doc/html/rfc9535
[idea-variables]: https://www.jetbrains.com/help/idea/http-client-variables.html
[idea-http]: https://www.jetbrains.com/help/idea/http-client-in-product-code-editor.html
[vscode-http]: https://github.com/Huachao/vscode-restclient#readme
[apifox-import]: https://docs.apifox.com/import-openapi-swagger.md
[apifox-options]: https://docs.apifox.com/import-settings.md
[openapi-diff]: https://github.com/OpenAPITools/openapi-diff#readme
[json-schema]: https://json-schema.org/understanding-json-schema/reference/object
[bruno-git]: https://docs.usebruno.com/git-integration/overview
[insomnia-git]: https://developer.konghq.com/insomnia/git-sync/
[oauth-client]: https://datatracker.ietf.org/doc/html/rfc6749#section-4.4
[oauth-native]: https://www.rfc-editor.org/rfc/rfc8252.html
[sse]: https://html.spec.whatwg.org/multipage/server-sent-events.html
[grpc]: https://grpc.io/docs/what-is-grpc/core-concepts/
[yaak-grpc]: https://yaak.app/docs/request-types/grpc-requests
