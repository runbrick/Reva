package i18n

// Import holds all copy and diagnostic mappings for the import dialog.
type Import struct {
	Title, Description, Input, Placeholder                                                        string
	LoadFile, JSONFiles, HARFiles, AllFiles, Submit, Cancel                                       string
	Unknown, InvalidCurl, InvalidPostman                                                          string
	FileError, SaveError, Success, EnvironmentSuccess, RequestSuccess, Loading                    string
	Review, Back, Parsing, Acknowledge, Warnings, NoWarnings                                      string
	Help, Destination, Counts, Format, NewRequest, NewEntity                                      string
	NativeCollection, NativeEnvironment, PostmanCollection, PostmanEnvironment, HAR, Curl         string
	InvalidJSON, UnsupportedFormat, UnsupportedVersion, AmbiguousFormat                           string
	InputTooLarge, LimitExceeded, InvalidStructure, InvalidAuth, UnsupportedAuth, UnsupportedBody string
	ScriptsOmitted, MetadataOmitted, ExamplesOmitted, PathVariablesUnresolved                     string
	DynamicVariablesUnsupported, LocalFileReferences, FormMetadataOmitted                         string
	TransportFlagsIgnored, HARResponseOmitted, HARCookiesOmitted                                  string
}

var importZh = Import{
	Title: "导入", Description: "先检查内容和兼容性提示，确认后再导入。",
	Input: "导入内容", Placeholder: "粘贴 curl、Reva / Postman JSON 或 HAR…",
	LoadFile: "打开文件", JSONFiles: "JSON 文件", HARFiles: "HAR 文件", AllFiles: "所有文件",
	Submit: "确认导入", Cancel: "取消", Review: "检查导入", Back: "返回编辑", Parsing: "正在检查内容…",
	Acknowledge: "我已阅读并接受以上提示", Warnings: "兼容性提示（%d）", NoWarnings: "未发现兼容性提示。",
	Help:   "支持 Reva v1 集合/环境、Postman v2.0/v2.1 集合/环境、HAR 1.2 和 curl（最多 32 MiB）。Apifox：设置 → 导出数据 → Postman；ApiPost：项目设置 → 导出项目 → Postman 2.1。不支持这两者的专有项目格式。",
	Format: "识别格式：%s", Destination: "目标：%s", Counts: "请求 %d · 文件夹 %d · 变量 %d",
	NewRequest: "新的未保存请求", NewEntity: "创建新项目，不覆盖已有内容；环境需手动启用。",
	NativeCollection: "Reva v1 集合", NativeEnvironment: "Reva v1 环境", PostmanCollection: "Postman 集合 v2.0/v2.1",
	PostmanEnvironment: "Postman 环境", HAR: "HAR 1.2 请求集合", Curl: "curl 请求",
	Unknown: "无法识别内容，请选择支持的格式。", InvalidCurl: "curl 命令无效或含不支持的选项，请检查引号、URL 和参数。",
	InvalidPostman: "Postman 集合无效，请检查 JSON 和请求结构。", FileError: "无法读取文件，请选择可读取的文件。",
	SaveError: "无法保存导入内容；检查存储位置后可重试。", Success: "集合导入成功", EnvironmentSuccess: "环境导入成功", RequestSuccess: "请求已打开", Loading: "正在读取文件…",
	InvalidJSON: "JSON 无效；请检查编码、重复字段及文档是否完整。", UnsupportedFormat: "不支持此格式。Apifox / ApiPost 请先导出为 Postman；不支持 OpenAPI、Insomnia、批量备份或 YAML。",
	UnsupportedVersion: "不支持此版本，请使用 Reva v1、Postman v2.0/v2.1 或 HAR 1.2。", AmbiguousFormat: "内容混合了多种格式标记，请一次导入一个集合或环境。",
	InputTooLarge: "内容超过 32 MiB，请缩小后重试。", LimitExceeded: "内容超过结构限制（256 层、10,000 个节点或 100,000 行）；请拆分导出。",
	InvalidStructure: "导入结构无效，请检查名称、字段类型和请求/文件夹结构。", InvalidAuth: "认证配置无效，请检查认证类型和 API Key 位置。",
	UnsupportedAuth: "不支持此认证方式。仅支持 Bearer、Basic、API Key、继承和无认证；请在源工具中调整后导出。", UnsupportedBody: "不支持此请求体或文件/二进制编码，请改用受支持的文本格式后导出。",
	ScriptsOmitted: "Postman 脚本已省略，请手动迁移并检查后再发送。", MetadataOmitted: "无法表示的备注、扩展或元数据已省略。", ExamplesOmitted: "保存的示例和响应已省略。",
	PathVariablesUnresolved: "请求局部路径变量保留为原文，请在发送前替换或配置。", DynamicVariablesUnsupported: "动态或 vault 变量保留为原文，Reva 不会自动解析。",
	LocalFileReferences: "本地文件路径仅保留为引用；未读取文件，发送前请检查并重新选择。", FormMetadataOmitted: "表单字段的部分说明或内容类型已省略。",
	TransportFlagsIgnored: "curl 的 TLS、重定向或压缩选项未应用；请检查请求选项。", HARResponseOmitted: "仅提取 HAR 请求；响应、页面和耗时不会导入历史记录。", HARCookiesOmitted: "HAR cookie 列表未转换为 Cookie 请求头，请在发送前补充。",
}

var importEn = Import{
	Title: "Import", Description: "Review content and compatibility notices before confirming the import.",
	Input: "Import content", Placeholder: "Paste curl, Reva / Postman JSON, or HAR…",
	LoadFile: "Open file", JSONFiles: "JSON files", HARFiles: "HAR files", AllFiles: "All files",
	Submit: "Confirm import", Cancel: "Cancel", Review: "Review import", Back: "Back to edit", Parsing: "Checking content…",
	Acknowledge: "I have read and accept these notices", Warnings: "Compatibility notices (%d)", NoWarnings: "No compatibility notices found.",
	Help:   "Reva v1 collections/environments, Postman v2.0/v2.1 collections/environments, HAR 1.2 and curl; up to 32 MiB. Apifox: Settings → Export data → Postman. ApiPost: Project settings → Export project → Postman 2.1. Their proprietary project formats are unsupported.",
	Format: "Detected format: %s", Destination: "Destination: %s", Counts: "Requests %d · Folders %d · Variables %d",
	NewRequest: "New unsaved request", NewEntity: "Creates a new entity without overwriting existing data. Activate environments manually.",
	NativeCollection: "Reva v1 collection", NativeEnvironment: "Reva v1 environment", PostmanCollection: "Postman collection v2.0/v2.1",
	PostmanEnvironment: "Postman environment", HAR: "HAR 1.2 request collection", Curl: "curl request",
	Unknown: "Unrecognized content. Choose a supported format.", InvalidCurl: "Invalid curl command or unsupported option. Check quoting, URL, and arguments.",
	InvalidPostman: "Invalid Postman collection. Check the JSON and request structure.", FileError: "Cannot read this file. Choose a readable file.",
	SaveError: "Cannot save the import. Check the storage location, then retry.", Success: "Collection imported", EnvironmentSuccess: "Environment imported", RequestSuccess: "Request opened", Loading: "Reading file…",
	InvalidJSON: "Invalid JSON. Check encoding, duplicate fields, and document completeness.", UnsupportedFormat: "Unsupported format. Export Apifox / ApiPost as Postman first; OpenAPI, Insomnia, bulk backups, and YAML are unsupported.",
	UnsupportedVersion: "Unsupported version. Use Reva v1, Postman v2.0/v2.1, or HAR 1.2.", AmbiguousFormat: "Content contains conflicting format markers. Import one collection or environment at a time.",
	InputTooLarge: "Content exceeds 32 MiB. Reduce it and retry.", LimitExceeded: "Content exceeds structural limits (256 levels, 10,000 nodes, or 100,000 rows). Split the export.",
	InvalidStructure: "Invalid import structure. Check names, field types, and request/folder shapes.", InvalidAuth: "Invalid authentication. Check the auth type and API-key location.",
	UnsupportedAuth: "Unsupported authentication. Bearer, Basic, API key, inheritance, and no auth are supported. Adjust the source before exporting.", UnsupportedBody: "Unsupported body or file/binary encoding. Export a supported text representation.",
	ScriptsOmitted: "Postman scripts were omitted. Migrate and review them manually before sending.", MetadataOmitted: "Notes, extensions, or metadata that cannot be represented were omitted.", ExamplesOmitted: "Saved examples and responses were omitted.",
	PathVariablesUnresolved: "Request-local path variables remain literal. Replace or configure them before sending.", DynamicVariablesUnsupported: "Dynamic or vault variables remain literal and will not be resolved automatically.",
	LocalFileReferences: "Local file paths remain references. No files were read; check and reselect them before sending.", FormMetadataOmitted: "Some form field descriptions or content types were omitted.",
	TransportFlagsIgnored: "curl TLS, redirect, or compression flags were not applied. Review request options.", HARResponseOmitted: "Only HAR requests are extracted. Responses, pages, and timings are not imported into history.", HARCookiesOmitted: "HAR cookie lists were not converted to a Cookie header. Supply one before sending.",
}

// ErrorMessage maps the parser's stable codes without exposing decoder text.
func (s Import) ErrorMessage(code string) string {
	switch code {
	case "invalid_json":
		return s.InvalidJSON
	case "unknown_format":
		return s.Unknown
	case "unsupported_format":
		return s.UnsupportedFormat
	case "unsupported_version":
		return s.UnsupportedVersion
	case "ambiguous_format":
		return s.AmbiguousFormat
	case "input_too_large":
		return s.InputTooLarge
	case "limit_exceeded":
		return s.LimitExceeded
	case "invalid_structure":
		return s.InvalidStructure
	case "invalid_auth":
		return s.InvalidAuth
	case "unsupported_auth":
		return s.UnsupportedAuth
	case "unsupported_body":
		return s.UnsupportedBody
	case "invalid_curl":
		return s.InvalidCurl
	}
	return s.Unknown
}

// WarningMessage maps every public fidelity diagnostic to human-facing copy.
func (s Import) WarningMessage(code string) string {
	switch code {
	case "scripts_omitted":
		return s.ScriptsOmitted
	case "metadata_omitted":
		return s.MetadataOmitted
	case "examples_omitted":
		return s.ExamplesOmitted
	case "path_variables_unresolved":
		return s.PathVariablesUnresolved
	case "dynamic_variables_unsupported":
		return s.DynamicVariablesUnsupported
	case "local_file_references":
		return s.LocalFileReferences
	case "form_metadata_omitted":
		return s.FormMetadataOmitted
	case "transport_flags_ignored":
		return s.TransportFlagsIgnored
	case "har_response_omitted":
		return s.HARResponseOmitted
	case "har_cookies_omitted":
		return s.HARCookiesOmitted
	}
	return s.MetadataOmitted
}
