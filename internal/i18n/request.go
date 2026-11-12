package i18n

// Request holds the strings of the request editor. Owned by that feature's PR.
type Request struct {
	Title, Method, Send, SendAccessible, Save, Cancel          string
	Params, Headers, Body, Auth, Scripts                       string
	None, Raw, FormData, URLEncoded, Binary                    string
	BodyText, ContentType, Format, InvalidJSON                 string
	Text, File, FilePath, Browse, Enabled                      string
	Basic, APIKey, Token, Username, Password                   string
	APIKeyName, APIKeyValue, APIKeyLocation, InHeader, InQuery string
	URL, SendMenu, SendDownload, CopyCURL, DownloadName        string
	Settings, Timeout, FollowRedirects, VerifyTLS              string
	Inherit, On, Off, TimeoutHelp                              string
	ParamName, ParamValue, HeaderName, HeaderValue             string
	Description, AddParam, AddHeader, BulkEdit, TableEdit      string
	BulkContent, BulkHelp                                      string
	GraphQLQuery, GraphQLVariables, GraphQLRaw, GraphQLEdit    string
}

var requestZh = Request{
	GraphQLQuery: "GraphQL 查询", GraphQLVariables: "GraphQL 变量（JSON）", GraphQLRaw: "编辑 JSON", GraphQLEdit: "编辑 GraphQL",
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
	GraphQLQuery: "GraphQL query", GraphQLVariables: "GraphQL variables (JSON)", GraphQLRaw: "Edit JSON", GraphQLEdit: "Edit GraphQL",
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
