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
}

var requestZh = Request{
	Title: "请求", Method: "方法", Send: "Send / 发送", SendAccessible: "Send / 发送", Save: "保存", Cancel: "取消",
	Params: "参数", Headers: "请求头", Body: "请求体", Auth: "认证", Scripts: "脚本",
	None: "无", Raw: "原始文本", FormData: "表单数据", URLEncoded: "网址编码表单", Binary: "二进制",
	BodyText: "请求体内容", ContentType: "内容类型", Format: "格式化", InvalidJSON: "JSON 格式无效",
	Text: "文本", File: "文件", FilePath: "文件路径", Browse: "浏览", Enabled: "启用",
	Basic: "基本认证", APIKey: "API 密钥", Token: "令牌", Username: "用户名", Password: "密码",
	APIKeyName: "密钥名称", APIKeyValue: "密钥值", APIKeyLocation: "密钥位置", InHeader: "请求头", InQuery: "查询参数",
}

var requestEn = Request{
	Title: "Request", Method: "Method", Send: "Send", SendAccessible: "Send", Save: "Save", Cancel: "Cancel",
	Params: "Params", Headers: "Headers", Body: "Body", Auth: "Auth", Scripts: "Scripts",
	None: "None", Raw: "Raw", FormData: "Form data", URLEncoded: "URL encoded", Binary: "Binary",
	BodyText: "Body content", ContentType: "Content type", Format: "Format", InvalidJSON: "Invalid JSON",
	Text: "Text", File: "File", FilePath: "File path", Browse: "Browse", Enabled: "Enabled",
	Basic: "Basic", APIKey: "API key", Token: "Token", Username: "Username", Password: "Password",
	APIKeyName: "API key name", APIKeyValue: "API key value", APIKeyLocation: "API key location", InHeader: "In header", InQuery: "In query",
}
