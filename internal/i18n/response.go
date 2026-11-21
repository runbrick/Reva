package i18n

// Response holds the strings of the response viewer. Owned by that feature's PR.
type Response struct {
	Title              string
	EmptyHint          string
	Sending            string
	ErrorTitle         string
	Success            string
	Received           string
	Duration           string
	Size               string
	Truncated          string
	Body               string
	Headers            string
	Tests              string
	TestsCount         string
	Pretty             string
	Raw                string
	Copy               string
	CopyError          string
	DisplayLimit       string
	HeaderName         string
	HeaderValue        string
	NoHeaders          string
	Cookies            string
	Timeline           string
	Preview            string
	Wrap               string
	CodeBody           string
	Search             string
	SearchHint         string
	NextMatch          string
	PreviousMatch      string
	CloseSearch        string
	MatchCount         string
	NoMatches          string
	NoCookies          string
	CookieDomain       string
	CookiePath         string
	CookieExpires      string
	CookieHTTPOnly     string
	CookieSecure       string
	Yes                string
	No                 string
	SessionCookie      string
	NoTimeline         string
	Total              string
	PhaseDNS           string
	PhaseConnect       string
	PhaseTLS           string
	PhaseSend          string
	PhaseWait          string
	PhaseDownload      string
	PhaseOther         string
	PhaseDuration      string
	PreviewUnavailable string
	EmptyPreview       string
	SSEEvents          string
	SSEConnected       string
	SSEDisconnect      string
	SSEDisconnected    string
	SSEEnded           string
	SSETimedOut        string
	SSEFailed          string
	SSEOverflow        string
	SSEEmpty           string
	SSEEventNumber     string
	SSEEventType       string
	SSEEventID         string
	Tools              ResponseTools
	Artifacts          ResponseArtifacts
}

type ResponseTools struct {
	Title, Compare, Fields, SetBaseline, ClearBaseline, RunComparison              string
	NoBaseline, BaselineReady, Baseline, Current, Ignored, IgnoreHint, IgnoreError string
	JSONUnavailable, NoChanges, MoreChanges, Status, Root, Missing                 string
	Added, Removed, TypeChanged, ValueChanged, InspectCurrent                      string
	Object, Array, String, Number, Boolean, Null                                   string
	Filter, FilterHint, NoFields, MoreFields, SelectField, Selected                string
	Exists, Equals, Capture, Variable, VariableHint, CaptureHelp, EqualityHelp     string
	ExistsTest, EqualsTest, CaptureTest, ScriptAdded                               string
}

type ResponseArtifacts struct {
	Example, Export, ExportExample, Empty, RequiresSaved, ReviewSave, ReviewReplace string
	Save, Replace, Delete, ConfirmDelete, DeleteHelp, Saved, Deleted, NoSavedBody   string
	Path, OptionalPath, PathError, SaveHelp, ExportHelp, Back                       string
	IncludeBody, BodyReview, BodyHelp, BodyUnavailable, TemplateBody                string
	Preview, JSONPreview, PreviewHelp, ConfirmExport, Exported                      string
	ReadError, WriteError, DeleteError, MappingError, ExportError                   string
}

var responseZh = Response{
	Artifacts: ResponseArtifacts{
		Example: "响应示例", Export: "导出 WireMock", ExportExample: "导出此示例",
		Empty: "此请求还没有响应示例。手动发送请求后即可保存示例。", RequiresSaved: "请先将请求保存到集合。",
		ReviewSave: "查看并保存当前响应", ReviewReplace: "查看并替换为当前响应", Save: "保存示例", Replace: "替换示例",
		Delete: "删除示例", ConfirmDelete: "确认删除示例", DeleteHelp: "确定要删除此请求的本地响应示例吗？",
		Saved: "已在本地保存示例", Deleted: "已删除本地示例", NoSavedBody: "此示例未保存正文。",
		Path: "固定匹配路径", OptionalPath: "固定匹配路径（可选）", PathError: "请输入以 / 开头的固定路径，不含主机、查询参数、片段或变量。",
		SaveHelp: "示例保存在本机。请移除路径中的密钥或敏感标识，也可以留空。", ExportHelp: "仅匹配此方法和路径。请检查路径并移除密钥或敏感标识。只有允许的 Content-Type 才会导出。",
		Back: "返回", IncludeBody: "包括响应正文", BodyReview: "响应正文检查", BodyHelp: "正文可能包含密钥或个人信息。继续前请检查完整正文。",
		BodyUnavailable: "没有可包含的完整 UTF-8 文本正文（上限 256 KiB）。", TemplateBody: "包含模板起始标记的正文不能用于静态映射。",
		Preview: "预览映射 JSON", JSONPreview: "WireMock 映射 JSON 预览", PreviewHelp: "请核对映射 JSON。确认后选择文件位置并保存。",
		ConfirmExport: "确认导出 JSON", Exported: "已导出 WireMock 映射", ReadError: "无法读取本地示例。", WriteError: "无法保存本地示例。",
		DeleteError: "无法删除本地示例。", MappingError: "无法生成映射。请检查路径和正文设置。", ExportError: "无法写入映射 JSON 文件。",
	},
	Tools: ResponseTools{
		Title: "响应工具", Compare: "响应对比", Fields: "JSON 字段", SetBaseline: "设为基准", ClearBaseline: "清除基准", RunComparison: "与基准对比",
		NoBaseline: "先将一份响应设为基准，再手动发送请求并回到此处对比。", BaselineReady: "基准已保存到当前会话。",
		Baseline: "基准响应", Current: "当前响应", Ignored: "忽略的路径", IgnoreHint: "每行一个 JSON Pointer，例如 /timestamp", IgnoreError: "请输入有效的 JSON Pointer，每行一个。",
		JSONUnavailable: "JSON 字段操作需要完整、无重复键的 JSON 正文（最大 512 KiB）。", NoChanges: "JSON 字段无差异。", MoreChanges: "这里只显示前 100 项差异。", Status: "HTTP 状态码", Root: "JSON 根值", Missing: "缺失",
		Added: "新增", Removed: "删除", TypeChanged: "类型变化", ValueChanged: "值变化", InspectCurrent: "选用当前字段",
		Object: "对象", Array: "数组", String: "字符串", Number: "数字", Boolean: "布尔值", Null: "空值",
		Filter: "筛选字段路径", FilterHint: "例如 /data/items/0", NoFields: "没有匹配的字段。", MoreFields: "这里显示前 64 项，可继续缩小筛选范围。", SelectField: "选择一个字段来生成断言或提取变量。", Selected: "已选字段",
		Exists: "添加存在断言", Equals: "添加等值断言", Capture: "添加提取规则", Variable: "变量名", VariableHint: "例如 orderId",
		CaptureHelp:  "保存请求后，集合运行中的后续请求可引用 {{变量名}}。提取值仅在本轮有效。",
		EqualityHelp: "等值断言会把当前字段值写入测试脚本。", ExistsTest: "字段存在 %s", EqualsTest: "字段值相等 %s", CaptureTest: "提取变量 %s",
		ScriptAdded: "已添加到请求的测试脚本",
	},
	SSEEvents: "SSE 事件", SSEConnected: "流已连接", SSEDisconnect: "断开连接", SSEDisconnected: "已断开连接",
	SSEEnded: "流已结束", SSETimedOut: "流已超时", SSEFailed: "读取事件流失败",
	SSEOverflow: "部分事件超出查看上限，无法显示。", SSEEmpty: "暂无完整事件。",
	SSEEventNumber: "事件 %d", SSEEventType: "类型：%s", SSEEventID: "ID：%s",
	Title: "响应", EmptyHint: "点击发送或按 Ctrl+Enter 查看响应。",
	Sending: "正在发送请求…", ErrorTitle: "请求失败", Success: "请求成功", Received: "接口响应已返回",
	Duration: "耗时", Size: "大小", Truncated: "响应内容已被截断。",
	Body: "响应", Headers: "响应头", Tests: "测试", TestsCount: "测试 (%d)",
	Pretty: "美化", Raw: "原生", Copy: "复制正文", CopyError: "无法复制正文",
	DisplayLimit: "仅显示正文的前 2 MiB；复制按钮可复制完整的已接收正文。",
	HeaderName:   "名称", HeaderValue: "值", NoHeaders: "没有响应头。",
	Cookies: "Cookie", Timeline: "时间线", Preview: "预览", Wrap: "自动换行", CodeBody: "响应正文",
	Search: "查找正文", SearchHint: "查找…", NextMatch: "下一个匹配", PreviousMatch: "上一个匹配", CloseSearch: "关闭查找", MatchCount: "%d / %d", NoMatches: "没有匹配项",
	NoCookies: "没有响应 Cookie。", CookieDomain: "域名", CookiePath: "路径", CookieExpires: "过期时间", CookieHTTPOnly: "HttpOnly", CookieSecure: "Secure",
	Yes: "是", No: "否", SessionCookie: "会话", NoTimeline: "没有时间线数据。", Total: "总耗时",
	PhaseDNS: "DNS 查询", PhaseConnect: "建立连接", PhaseTLS: "TLS 握手", PhaseSend: "发送请求", PhaseWait: "等待响应", PhaseDownload: "下载响应", PhaseOther: "其他",
	PhaseDuration:      "%.2f ms",
	PreviewUnavailable: "此响应无法预览。", EmptyPreview: "HTML 中没有可显示的文本。",
}

var responseEn = Response{
	Artifacts: ResponseArtifacts{
		Example: "Response example", Export: "Export WireMock", ExportExample: "Export this example",
		Empty: "This request has no saved example. Send it manually to capture one.", RequiresSaved: "Save the request to a collection first.",
		ReviewSave: "Review current response", ReviewReplace: "Review replacement response", Save: "Save example", Replace: "Replace example",
		Delete: "Delete example", ConfirmDelete: "Confirm example deletion", DeleteHelp: "Delete the local response example for this request?",
		Saved: "Response example saved locally", Deleted: "Local response example deleted", NoSavedBody: "This example has no saved body.",
		Path: "Exact match path", OptionalPath: "Exact match path (optional)", PathError: "Enter a fixed path starting with /. Leave out hosts, queries, fragments and variables.",
		SaveHelp:   "The example stays on this device. Remove secrets or sensitive identifiers from the path, or leave it blank.",
		ExportHelp: "WireMock matches this method and path. Remove secrets or sensitive identifiers from the path. Only allowlisted Content-Type values are exported.",
		Back:       "Back", IncludeBody: "Include response body", BodyReview: "Review response body", BodyHelp: "Review the full body for secrets or personal information before continuing.",
		BodyUnavailable: "There is no complete UTF-8 text body available to include (256 KiB limit).", TemplateBody: "Static mappings omit bodies that contain a template opening delimiter.",
		Preview: "Preview mapping JSON", JSONPreview: "WireMock mapping JSON preview", PreviewHelp: "Review the exact mapping JSON. Confirm to choose a location and save it.",
		ConfirmExport: "Confirm JSON export", Exported: "WireMock mapping exported", ReadError: "Could not read the local example.",
		WriteError: "Could not save the local example.", DeleteError: "Could not delete the local example.",
		MappingError: "Could not build the mapping. Check the path and body options.", ExportError: "Could not write the mapping JSON file.",
	},
	Tools: ResponseTools{
		Title: "Response tools", Compare: "Compare responses", Fields: "JSON fields", SetBaseline: "Set as baseline", ClearBaseline: "Clear baseline", RunComparison: "Compare with baseline",
		NoBaseline: "Set a baseline, send another request manually, then return here to compare.", BaselineReady: "The baseline is available for this session.",
		Baseline: "Baseline response", Current: "Current response", Ignored: "Ignored paths", IgnoreHint: "One JSON Pointer per line, e.g. /timestamp", IgnoreError: "Enter one valid JSON Pointer per line.",
		JSONUnavailable: "JSON field actions require a complete JSON body without duplicate keys (up to 512 KiB).", NoChanges: "No JSON field differences.", MoreChanges: "Showing the first 100 differences.", Status: "HTTP status", Root: "JSON root", Missing: "Missing",
		Added: "Added", Removed: "Removed", TypeChanged: "Type changed", ValueChanged: "Value changed", InspectCurrent: "Use current field",
		Object: "Object", Array: "Array", String: "String", Number: "Number", Boolean: "Boolean", Null: "Null",
		Filter: "Filter field paths", FilterHint: "e.g. /data/items/0", NoFields: "No matching fields.", MoreFields: "Showing the first 64 matches. Narrow the filter to find more.", SelectField: "Select a field to generate an assertion or capture a variable.", Selected: "Selected field",
		Exists: "Add existence check", Equals: "Add equality check", Capture: "Add capture rule", Variable: "Variable name", VariableHint: "e.g. orderId",
		CaptureHelp:  "Save the request to use {{variableName}} in later requests in a collection run. Captured values last for that run.",
		EqualityHelp: "An equality check writes the current field value into the test script.", ExistsTest: "JSON field exists %s", EqualsTest: "JSON field equals %s", CaptureTest: "Capture variable %s",
		ScriptAdded: "Added to the request test script",
	},
	SSEEvents: "SSE events", SSEConnected: "Stream connected", SSEDisconnect: "Disconnect", SSEDisconnected: "Disconnected",
	SSEEnded: "Stream ended", SSETimedOut: "Stream timed out", SSEFailed: "Could not read the event stream",
	SSEOverflow: "Some events exceeded the inspection limits and could not be shown.", SSEEmpty: "No complete events yet.",
	SSEEventNumber: "Event %d", SSEEventType: "Type: %s", SSEEventID: "ID: %s",
	Title: "Response", EmptyHint: "Press Send or Ctrl+Enter to view a response.",
	Sending: "Sending request…", ErrorTitle: "Request failed", Success: "Request succeeded", Received: "Response received",
	Duration: "Time", Size: "Size", Truncated: "The response body was truncated.",
	Body: "Response", Headers: "Headers", Tests: "Tests", TestsCount: "Tests (%d)",
	Pretty: "Pretty", Raw: "Raw", Copy: "Copy body", CopyError: "Could not copy body",
	DisplayLimit: "Showing only the first 2 MiB of the body; Copy copies the full received body.",
	HeaderName:   "Name", HeaderValue: "Value", NoHeaders: "No response headers.",
	Cookies: "Cookies", Timeline: "Timeline", Preview: "Preview", Wrap: "Word wrap", CodeBody: "Response body",
	Search: "Find in body", SearchHint: "Find…", NextMatch: "Next match", PreviousMatch: "Previous match", CloseSearch: "Close find", MatchCount: "%d / %d", NoMatches: "No matches",
	NoCookies: "No response cookies.", CookieDomain: "Domain", CookiePath: "Path", CookieExpires: "Expires", CookieHTTPOnly: "HttpOnly", CookieSecure: "Secure",
	Yes: "Yes", No: "No", SessionCookie: "Session", NoTimeline: "No timeline data.", Total: "Total",
	PhaseDNS: "DNS lookup", PhaseConnect: "Connect", PhaseTLS: "TLS handshake", PhaseSend: "Send request", PhaseWait: "Wait for response", PhaseDownload: "Download", PhaseOther: "Other",
	PhaseDuration:      "%.2f ms",
	PreviewUnavailable: "Preview is not available for this response.", EmptyPreview: "This HTML has no visible text.",
}
