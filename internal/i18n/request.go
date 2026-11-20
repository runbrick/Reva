package i18n

// Request holds the strings of the request editor. Owned by that feature's PR.
type Request struct {
	Preview, PreviewTitle, PreviewClose, PreviewPhase, PreviewLimits                      string
	PreviewMaskHelp, PreviewMasked, PreviewURLInvalid                                     string
	PreviewVariables, PreviewVariableRow, PreviewNoVariables                              string
	PreviewCollection, PreviewEnvironment, PreviewActiveEnvironment, PreviewNoEnvironment string
	PreviewWinner, PreviewShadowed, PreviewDisabled                                       string
	PreviewUnresolved, PreviewNoUnresolved, PreviewOwn, PreviewFolder                     string
	PreviewAuth, PreviewAuthOverride, PreviewAuthQuery, PreviewNoHeaders, PreviewBody     string
	Bearer, AuthInherit, AuthMissingContext, AuthParentNone                               string
	AuthFromCollection, AuthFromFolder, AuthError, AuthInvalid                            string
	CURLFailed, AuthUnsupported                                                           string
	Title, Method, Send, SendAccessible, Save, Cancel                                     string
	Params, Headers, Body, Auth, Scripts                                                  string
	None, Raw, FormData, URLEncoded, Binary                                               string
	BodyText, ContentType, Format, InvalidJSON                                            string
	Text, File, FilePath, Browse, Enabled                                                 string
	Basic, APIKey, Token, Username, Password                                              string
	APIKeyName, APIKeyValue, APIKeyLocation, InHeader, InQuery                            string
	URL, SendMenu, SendDownload, CopyCURL, DownloadName                                   string
	Settings, Timeout, FollowRedirects, VerifyTLS                                         string
	CookieSessionEnabled, CookieSessionClear, CookieSessionHistoryError                   string
	Inherit, On, Off, TimeoutHelp                                                         string
	ParamName, ParamValue, HeaderName, HeaderValue                                        string
	Description, AddParam, AddHeader, BulkEdit, TableEdit                                 string
	BulkContent, BulkHelp                                                                 string
	GraphQLQuery, GraphQLVariables, GraphQLRaw, GraphQLEdit                               string
	Details, QueryParameters                                                              string
}

var requestZh = Request{
	CookieSessionEnabled: "使用临时 Cookie 会话", CookieSessionClear: "清空手动 Cookie 会话", CookieSessionHistoryError: "请求失败（未保存错误详情）",
	Details: "接口信息", QueryParameters: "查询参数",
	Preview: "请求预览", PreviewTitle: "发送前只读预览", PreviewClose: "关闭预览",
	PreviewPhase:    "在前置脚本之前计算；按当前请求草稿、已保存的集合和活动环境实时刷新。",
	PreviewLimits:   "这不是完整的线上报文：不运行脚本、不发送、不保存、不读取上传文件；不计算网络库自动生成的请求头、请求体编码或重定向。WebSocket 不使用此 HTTP 预览。",
	PreviewMaskHelp: "所有变量值均隐藏，包括主机和路径变量；URL 仅显示字面路由结构。查询值、请求头值、凭据、片段及请求体全部隐藏；[masked] 表示已隐藏。未知变量名称列于下方。",
	PreviewMasked:   "值已隐藏", PreviewURLInvalid: "无法预览 URL 或查询参数，请检查 URL（错误详情已隐藏）。",
	PreviewVariables: "变量来源与覆盖", PreviewVariableRow: "%s · %s 第 %d 行 · %s · 值已隐藏", PreviewNoVariables: "没有集合或环境变量。",
	PreviewCollection: "集合", PreviewEnvironment: "环境", PreviewActiveEnvironment: "活动环境：%s", PreviewNoEnvironment: "未选择活动环境",
	PreviewWinner: "生效", PreviewShadowed: "被后面的启用行覆盖", PreviewDisabled: "禁用（不参与覆盖）",
	PreviewUnresolved: "未解析的变量", PreviewNoUnresolved: "没有未解析的变量。", PreviewOwn: "当前请求", PreviewFolder: "最近的父文件夹",
	PreviewAuth: "认证：%s · 来源：%s · 凭据已隐藏", PreviewAuthOverride: "显式请求头优先于生成的认证（空值也优先）。",
	PreviewAuthQuery: "查询密钥替换同名查询值；仅用于初始 URL。", PreviewNoHeaders: "没有显式或认证生成的请求头。", PreviewBody: "请求体：%s · 内容和文件路径已隐藏，未读取文件。",
	Bearer: "Bearer", AuthInherit: "继承父级认证", AuthMissingContext: "无法继承认证：所属集合或请求项已不存在，请重新打开请求或选择独立认证。",
	AuthParentNone: "父级已禁用认证，不会自动添加凭据。", AuthFromCollection: "%s · 来自集合「%s」", AuthFromFolder: "%s · 来自文件夹「%s」",
	AuthError: "认证错误", AuthInvalid: "认证配置无效，请检查认证类型、密钥位置及请求头名称和值。", CURLFailed: "无法生成 cURL，请检查 URL 查询参数。",
	AuthUnsupported: "不支持的认证类型",
	GraphQLQuery:    "GraphQL 查询", GraphQLVariables: "GraphQL 变量（JSON）", GraphQLRaw: "编辑 JSON", GraphQLEdit: "编辑 GraphQL",
	Title: "请求", Method: "方法", Send: "Send / 发送", SendAccessible: "Send / 发送", Save: "保存", Cancel: "取消",
	Params: "参数", Headers: "请求头", Body: "请求体", Auth: "认证", Scripts: "脚本",
	None: "无", Raw: "原始文本", FormData: "表单数据", URLEncoded: "网址编码表单", Binary: "二进制",
	BodyText: "请求体内容", ContentType: "内容类型", Format: "格式化", InvalidJSON: "JSON 格式无效",
	Text: "文本", File: "文件", FilePath: "文件路径", Browse: "浏览", Enabled: "启用",
	Basic: "基本认证", APIKey: "API 密钥", Token: "令牌", Username: "用户名", Password: "密码",
	APIKeyName: "密钥名称", APIKeyValue: "密钥值", APIKeyLocation: "密钥位置", InHeader: "请求头", InQuery: "查询参数",
	URL: "URL", SendMenu: "发送选项", SendDownload: "发送并下载", CopyCURL: "复制为 cURL", DownloadName: "响应.bin",
	Settings: "请求设置", Timeout: "超时（毫秒）", FollowRedirects: "跟随重定向", VerifyTLS: "验证 TLS 证书",
	Inherit: "继承", On: "开启", Off: "关闭", TimeoutHelp: "留空以继承全局设置；输入大于 0 的毫秒数。",
	ParamName: "参数名", ParamValue: "参数值", HeaderName: "请求头名称", HeaderValue: "请求头值",
	Description: "描述", AddParam: "添加参数", AddHeader: "添加请求头", BulkEdit: "批量编辑", TableEdit: "表格编辑",
	BulkContent: "批量参数", BulkHelp: "每行 key: value，禁用的行以 // 开头。描述保留在表格中。",
}

var requestEn = Request{
	CookieSessionEnabled: "Use temporary cookie sessions", CookieSessionClear: "Clear manual cookie sessions", CookieSessionHistoryError: "Request failed (details not saved)",
	Details: "Request details", QueryParameters: "Query parameters",
	Preview: "Request preview", PreviewTitle: "Read-only pre-send preview", PreviewClose: "Close preview",
	PreviewPhase:    "Computed BEFORE pre-request scripts; refreshed from the current draft, stored collection and active environment.",
	PreviewLimits:   "This is not a complete wire request: no scripts, sending, saving or upload file reads. Generated transport headers, body encoding and redirects are not evaluated. WebSocket does not use this HTTP preview.",
	PreviewMaskHelp: "All variable values are hidden, including host and path variables; the URL shows literal routing structure only. Query values, header values, credentials, fragments and body content are hidden; [masked] marks hidden text. Unknown variable names appear below.",
	PreviewMasked:   "Values hidden", PreviewURLInvalid: "Cannot preview the URL or query parameters. Check the URL (error details hidden).",
	PreviewVariables: "Variable sources and overrides", PreviewVariableRow: "%s · %s row %d · %s · value hidden", PreviewNoVariables: "No collection or environment variables.",
	PreviewCollection: "Collection", PreviewEnvironment: "Environment", PreviewActiveEnvironment: "Active environment: %s", PreviewNoEnvironment: "No active environment",
	PreviewWinner: "Winner", PreviewShadowed: "Shadowed by a later enabled row", PreviewDisabled: "Disabled (does not override)",
	PreviewUnresolved: "Unresolved variables", PreviewNoUnresolved: "No unresolved variables.", PreviewOwn: "Current request", PreviewFolder: "Nearest parent folder",
	PreviewAuth: "Authentication: %s · Source: %s · credentials hidden", PreviewAuthOverride: "An explicit header takes precedence over generated authentication, including an empty value.",
	PreviewAuthQuery: "The query API key replaces matching query values and applies only to the initial URL.", PreviewNoHeaders: "No explicit or authentication-generated headers.", PreviewBody: "Body: %s · content and file paths hidden; files have not been read.",
	Bearer: "Bearer", AuthInherit: "Inherit", AuthMissingContext: "Cannot inherit authentication: the collection or request item is missing. Reopen the request or select its own authentication.",
	AuthParentNone: "Authentication is disabled by the parent; no credentials will be generated.", AuthFromCollection: "%s · From collection “%s”", AuthFromFolder: "%s · From folder “%s”",
	AuthError: "Authentication error", AuthInvalid: "Invalid authentication configuration. Check the scheme, API key placement, and header name and value.", CURLFailed: "Cannot generate cURL. Check the URL query parameters.",
	AuthUnsupported: "Unsupported authentication",
	GraphQLQuery:    "GraphQL query", GraphQLVariables: "GraphQL variables (JSON)", GraphQLRaw: "Edit JSON", GraphQLEdit: "Edit GraphQL",
	Title: "Request", Method: "Method", Send: "Send", SendAccessible: "Send", Save: "Save", Cancel: "Cancel",
	Params: "Params", Headers: "Headers", Body: "Body", Auth: "Authorization", Scripts: "Scripts",
	None: "None", Raw: "Raw", FormData: "Form data", URLEncoded: "URL encoded", Binary: "Binary",
	BodyText: "Body content", ContentType: "Content type", Format: "Format", InvalidJSON: "Invalid JSON",
	Text: "Text", File: "File", FilePath: "File path", Browse: "Browse", Enabled: "Enabled",
	Basic: "Basic", APIKey: "API key", Token: "Token", Username: "Username", Password: "Password",
	APIKeyName: "API key name", APIKeyValue: "API key value", APIKeyLocation: "API key location", InHeader: "In header", InQuery: "In query",
	URL: "URL", SendMenu: "Send options", SendDownload: "Send and download", CopyCURL: "Copy as cURL", DownloadName: "response.bin",
	Settings: "Request settings", Timeout: "Timeout (ms)", FollowRedirects: "Follow redirects", VerifyTLS: "Verify TLS",
	Inherit: "Inherit", On: "On", Off: "Off", TimeoutHelp: "Leave empty to inherit global settings; enter a positive number of milliseconds.",
	ParamName: "Parameter name", ParamValue: "Parameter value", HeaderName: "Header name", HeaderValue: "Header value",
	Description: "Description", AddParam: "Add parameter", AddHeader: "Add header", BulkEdit: "Bulk edit", TableEdit: "Table edit",
	BulkContent: "Bulk parameters", BulkHelp: "One key: value per line; prefix disabled rows with //. Descriptions stay in the table.",
}
