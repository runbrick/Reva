package i18n

// Import holds all copy and diagnostic mappings for the import dialog.
type Import struct {
	ManualTitle, Sources, AutoSource, NativeSource, PostmanSource, ApifoxSource, ApiPostSource                 string
	SwaggerSource, SwaggerURLSource, YAPISource, CurlSource, MarkdownSource, HARSource                         string
	InsomniaSource, APIDocSource, ApizzaSource, EolinkSource, UnsupportedSource, MoreSources                   string
	Location, CurrentCollection, NewCollection, SelectCollection, NoCollection, CurrentHelp                    string
	PathHandling, AddBasePath, AddHost, KeepPaths, SelectFile, PasteContent, Details, UploadHint               string
	AutoHelp, NativeHelp, PostmanHelp, ApifoxHelp, ApiPostHelp, CurlHelp, HARHelp, FileLoaded                  string
	SourceMismatch, EnvironmentDestination, MissingDestination, VariableConflict, AppendDestination            string
	SwaggerHelp, SwaggerURLHelp, SchemaURL, SchemaURLPlaceholder, SwaggerCollection, OpenAPICollection         string
	InvalidSourceURL, FetchFailed, UnsupportedReference, UnsupportedParameter, OpenAPIReview, OpenAPISelection string
	OpenAPIEmptyServer, OpenAPISchemaChoice, OpenAPISchemaIncomplete, OpenAPINonstandardPath                   string

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
	ManualTitle: "导入项目（手动）", Sources: "数据来源", AutoSource: "自动识别", NativeSource: "Reva",
	PostmanSource: "Postman", ApifoxSource: "Apifox", ApiPostSource: "Apipost", SwaggerSource: "Swagger", SwaggerURLSource: "Swagger URL",
	YAPISource: "YApi", CurlSource: "cURL 导入", MarkdownSource: ".md 文件", HARSource: ".har 文件", InsomniaSource: "Insomnia",
	APIDocSource: "Apidoc", ApizzaSource: "Apizza", EolinkSource: "Eolink", UnsupportedSource: "暂不支持此来源，请使用其他受支持的格式。", MoreSources: "滚动查看更多来源",
	Location: "导入位置", CurrentCollection: "导入已有集合", NewCollection: "作为新项目", SelectCollection: "目标集合", NoCollection: "请先创建一个集合。",
	CurrentHelp:  "以独立文件夹追加，变量共享到目标集合；同名变量冲突时请作为新项目导入。",
	PathHandling: "路径处理方式", AddBasePath: "接口路径加上 basePath", AddHost: "接口路径加上 host", KeepPaths: "仅影响 Swagger / OpenAPI：basePath 使用基础路径，host 使用协议和主机；关闭时仅保留接口路径。其他格式保留原始 URL。",
	SelectFile: "选择文件", PasteContent: "粘贴内容", Details: "查看详细说明", UploadHint: "点击上传文件，或切换到「粘贴内容」；最大 32 MiB。",
	AutoHelp: "自动识别 Swagger / OpenAPI JSON、Reva、Postman、HAR 或 cURL；一次导入一个集合、环境或请求。", NativeHelp: "支持 Reva v1 集合和环境 JSON 文件。",
	SwaggerHelp:    "支持 Swagger 2.0、OpenAPI 3.0 / 3.1 JSON；选择文件或粘贴内容，仅解析文档内的引用。暂不支持 YAML。",
	SwaggerURLHelp: "输入 Swagger / OpenAPI JSON 文档的 HTTP(S) 地址（不是 Swagger UI 页面）。点击「检查导入」后获取，最长 30 秒、最多 32 MiB；不会获取外部引用。",
	SchemaURL:      "文档 URL", SchemaURLPlaceholder: "https://api.example.com/openapi.json",
	SwaggerCollection: "Swagger 2.0 集合", OpenAPICollection: "OpenAPI 3.0 / 3.1 集合",
	OpenAPIEmptyServer:      "所选服务器地址为空，按相对路径解析（URL 导入以文档地址为基准）；未切换到其他环境。发送前请检查并补充主机地址。",
	OpenAPISchemaIncomplete: "请求体 Schema 引用了文档中缺失的定义，该部分使用 null 占位；接口已保留，发送前请根据实际 API 补全请求体。",
	OpenAPINonstandardPath:  "路径字段使用了完整地址、主机端口或基础 URL 变量，已保留原文且不拼接服务器地址；发送前请检查协议、主机和变量。",
	OpenAPISchemaChoice:     "请求体包含 oneOf / anyOf，已使用第一个分支生成示例；请根据实际业务选择并修改，请求示例不保证满足 Schema 校验。",
	InvalidSourceURL:        "文档地址无效，请输入不含用户名、密码或片段的 HTTP(S) URL。", FetchFailed: "无法获取文档，请检查地址、网络和服务状态后重试；需要可直接访问的 JSON 文档。",
	UnsupportedReference: "引用无法解析：仅支持文档内的有效 JSON Pointer；请先合并外部引用并消除循环引用。", UnsupportedParameter: "不支持此参数位置或序列化方式，请改用简单路径、查询或请求头参数。",
	OpenAPIReview: "接口定义已转换为请求模板。请检查生成的示例值、路径变量并补充认证凭据后再发送；响应、文档、Schema 校验约束、回调和扩展不保留。", OpenAPISelection: "存在多个服务器、媒体类型、示例或认证方案，仅使用第一个（请求体优先 application/json）；请检查选择结果。",
	PostmanHelp: "支持 Postman v2.0 / v2.1 集合与单个环境的 JSON 导出文件。", ApifoxHelp: "请在 Apifox 中选择「设置 → 导出数据 → Postman」，再上传导出的 JSON；不支持专有项目格式。",
	ApiPostHelp: "请在 ApiPost 中选择「项目设置 → 导出项目 → Postman 2.1」，再上传导出的 JSON；不支持专有项目格式。",
	CurlHelp:    "粘贴单条 cURL 命令；仅解析请求，不会执行命令或读取引用的文件。", HARHelp: "支持 HAR 1.2 文件；仅提取请求，响应和耗时不会导入历史记录。",
	FileLoaded: "已选择：%s · %.1f KiB", SourceMismatch: "内容与所选数据来源不符，请选择对应来源或「自动识别」。",
	EnvironmentDestination: "环境文件请「作为新项目」导入，导入后可手动启用。", MissingDestination: "目标集合已不存在，请重新选择导入位置。",
	VariableConflict: "目标集合存在值或启用状态不同的同名变量，请「作为新项目」导入。", AppendDestination: "追加到集合：%s（独立文件夹）",
	Title: "导入", Description: "先检查内容和兼容性提示，确认后再导入。",
	Input: "导入内容", Placeholder: "粘贴 Swagger / OpenAPI JSON、curl、Reva / Postman JSON 或 HAR…",
	LoadFile: "打开文件", JSONFiles: "JSON 文件", HARFiles: "HAR 文件", AllFiles: "所有文件",
	Submit: "确认导入", Cancel: "取消", Review: "检查导入", Back: "返回编辑", Parsing: "正在检查内容…",
	Acknowledge: "我已阅读并接受以上提示", Warnings: "兼容性提示（%d）", NoWarnings: "未发现兼容性提示。",
	Help:   "支持 Swagger 2.0 / OpenAPI 3.0/3.1 JSON、Reva v1 集合/环境、Postman v2.0/v2.1 集合/环境、HAR 1.2 和 curl（最多 32 MiB）。Swagger URL 仅获取 JSON 文档，不获取外部引用；暂不支持 YAML。Apifox：设置 → 导出数据 → Postman；ApiPost：项目设置 → 导出项目 → Postman 2.1。不支持这两者的专有项目格式。",
	Format: "识别格式：%s", Destination: "目标：%s", Counts: "请求 %d · 文件夹 %d · 变量 %d",
	NewRequest: "新的未保存请求", NewEntity: "创建新项目，不覆盖已有内容；环境需手动启用。",
	NativeCollection: "Reva v1 集合", NativeEnvironment: "Reva v1 环境", PostmanCollection: "Postman 集合 v2.0/v2.1",
	PostmanEnvironment: "Postman 环境", HAR: "HAR 1.2 请求集合", Curl: "curl 请求",
	Unknown: "无法识别内容，请选择支持的格式。", InvalidCurl: "curl 命令无效或含不支持的选项，请检查引号、URL 和参数。",
	InvalidPostman: "Postman 集合无效，请检查 JSON 和请求结构。", FileError: "无法读取文件，请选择可读取的文件。",
	SaveError: "无法保存导入内容；检查存储位置后可重试。", Success: "集合导入成功", EnvironmentSuccess: "环境导入成功", RequestSuccess: "请求已打开", Loading: "正在读取文件…",
	InvalidJSON: "JSON 无效；请检查编码、重复字段及文档是否完整。", UnsupportedFormat: "不支持此格式。Apifox / ApiPost 请先导出为 Postman；不支持 Insomnia、批量备份或 YAML。",
	UnsupportedVersion: "不支持此版本，请使用 Swagger 2.0、OpenAPI 3.0/3.1、Reva v1、Postman v2.0/v2.1 或 HAR 1.2。", AmbiguousFormat: "内容混合了多种格式标记，请一次导入一个集合或环境。",
	InputTooLarge: "内容超过 32 MiB，请缩小后重试。", LimitExceeded: "内容超过结构限制（256 层、10,000 个节点或 100,000 行）；请拆分导出。",
	InvalidStructure: "导入结构无效，请检查名称、字段类型和请求/文件夹结构。", InvalidAuth: "认证配置无效，请检查认证类型和 API Key 位置。",
	UnsupportedAuth: "不支持此认证方式。仅支持 Bearer、Basic、API Key、继承和无认证；请在源工具中调整后导出。", UnsupportedBody: "不支持此请求体或文件/二进制编码，请改用受支持的文本格式后导出。",
	ScriptsOmitted: "Postman 脚本已省略，请手动迁移并检查后再发送。", MetadataOmitted: "无法表示的备注、扩展或元数据已省略。", ExamplesOmitted: "保存的示例和响应已省略。",
	PathVariablesUnresolved: "请求局部路径变量保留为原文，请在发送前替换或配置。", DynamicVariablesUnsupported: "动态或 vault 变量保留为原文，Reva 不会自动解析。",
	LocalFileReferences: "本地文件路径仅保留为引用；未读取文件，发送前请检查并重新选择。", FormMetadataOmitted: "表单字段的部分说明或内容类型已省略。",
	TransportFlagsIgnored: "curl 的 TLS、重定向或压缩选项未应用；请检查请求选项。", HARResponseOmitted: "仅提取 HAR 请求；响应、页面和耗时不会导入历史记录。", HARCookiesOmitted: "HAR cookie 列表未转换为 Cookie 请求头，请在发送前补充。",
}

var importEn = Import{
	ManualTitle: "Import project (manual)", Sources: "Data source", AutoSource: "Auto detect", NativeSource: "Reva",
	PostmanSource: "Postman", ApifoxSource: "Apifox", ApiPostSource: "Apipost", SwaggerSource: "Swagger", SwaggerURLSource: "Swagger URL",
	YAPISource: "YApi", CurlSource: "cURL import", MarkdownSource: ".md file", HARSource: ".har file", InsomniaSource: "Insomnia",
	APIDocSource: "Apidoc", ApizzaSource: "Apizza", EolinkSource: "Eolink", UnsupportedSource: "This source is not supported yet. Choose a supported format.", MoreSources: "Scroll for more sources",
	Location: "Import location", CurrentCollection: "Existing collection", NewCollection: "As a new project", SelectCollection: "Destination collection", NoCollection: "Create a collection first.",
	CurrentHelp:  "Append in a separate folder and share variables with the destination. Use a new project if variable values conflict.",
	PathHandling: "Path handling", AddBasePath: "Add basePath to paths", AddHost: "Add host to paths", KeepPaths: "Swagger / OpenAPI only: basePath adds the base path, host adds the scheme and host. When off, keep operation paths only. Other formats retain their original URLs.",
	SelectFile: "Select file", PasteContent: "Paste content", Details: "View instructions", UploadHint: "Click to upload a file, or switch to Paste content. Up to 32 MiB.",
	AutoHelp: "Detect Swagger / OpenAPI JSON, Reva, Postman, HAR or cURL. Import one collection, environment or request at a time.", NativeHelp: "Supports Reva v1 collection and environment JSON files.",
	SwaggerHelp:    "Supports Swagger 2.0 and OpenAPI 3.0 / 3.1 JSON. Open a file or paste content. Only in-document references are resolved. YAML is not supported yet.",
	SwaggerURLHelp: "Enter the HTTP(S) URL of a Swagger / OpenAPI JSON document, rather than its Swagger UI page. Review import fetches it with a 30-second timeout and 32 MiB limit. External references are never fetched.",
	SchemaURL:      "Document URL", SchemaURLPlaceholder: "https://api.example.com/openapi.json",
	SwaggerCollection: "Swagger 2.0 collection", OpenAPICollection: "OpenAPI 3.0 / 3.1 collection",
	OpenAPIEmptyServer:      "The selected server URL is empty. Paths are relative, using the document address for URL imports; another environment was not selected. Check and supply the host before sending.",
	OpenAPISchemaIncomplete: "A request-body schema references a definition missing from this document. That part uses a null placeholder. The operation is retained; complete its body against the actual API before sending.",
	OpenAPINonstandardPath:  "A path contains a full address, host and port, or base URL variable. It was preserved without a server prefix. Check its scheme, host and variables before sending.",
	OpenAPISchemaChoice:     "The body contains oneOf / anyOf. An example was generated from the first branch. Choose and edit it for your use case; the example is not guaranteed to satisfy schema validation.",
	InvalidSourceURL:        "Invalid document address. Enter an HTTP(S) URL without a username, password or fragment.", FetchFailed: "Cannot fetch the document. Check the address, network and server, then retry. A directly accessible JSON document is required.",
	UnsupportedReference: "Cannot resolve this reference. Only valid in-document JSON Pointers are supported. Bundle external references and remove reference cycles first.", UnsupportedParameter: "Unsupported parameter location or serialization. Use simple path, query or header parameters.",
	OpenAPIReview: "API definitions became request templates. Check generated examples and path variables and supply credentials before sending. Responses, documentation, schema validation constraints, callbacks and extensions are omitted.", OpenAPISelection: "Multiple servers, media types, examples or security alternatives exist. Only the first is used (request bodies prefer application/json). Review the selected result.",
	PostmanHelp: "Supports Postman v2.0 / v2.1 collection and single environment JSON exports.", ApifoxHelp: "In Apifox, choose Settings → Export data → Postman, then upload the JSON export. Proprietary project formats are unsupported.",
	ApiPostHelp: "In ApiPost, choose Project settings → Export project → Postman 2.1, then upload the JSON export. Proprietary project formats are unsupported.",
	CurlHelp:    "Paste one cURL command. Import only parses the request; it never executes commands or reads referenced files.", HARHelp: "Supports HAR 1.2 files. Only requests are extracted; responses and timings are not restored as history.",
	FileLoaded: "Selected: %s · %.1f KiB", SourceMismatch: "Content does not match the selected source. Choose the matching source or Auto detect.",
	EnvironmentDestination: "Import environment files as a new project, then activate them manually.", MissingDestination: "The destination collection no longer exists. Choose an import location again.",
	VariableConflict: "The destination has a variable with a different value or enabled state. Import as a new project.", AppendDestination: "Append to collection: %s (separate folder)",
	Title: "Import", Description: "Review content and compatibility notices before confirming the import.",
	Input: "Import content", Placeholder: "Paste Swagger / OpenAPI JSON, curl, Reva / Postman JSON, or HAR…",
	LoadFile: "Open file", JSONFiles: "JSON files", HARFiles: "HAR files", AllFiles: "All files",
	Submit: "Confirm import", Cancel: "Cancel", Review: "Review import", Back: "Back to edit", Parsing: "Checking content…",
	Acknowledge: "I have read and accept these notices", Warnings: "Compatibility notices (%d)", NoWarnings: "No compatibility notices found.",
	Help:   "Swagger 2.0 / OpenAPI 3.0/3.1 JSON; Reva v1; Postman v2.0/v2.1; HAR 1.2; curl. Max 32 MiB. URL: JSON only. No YAML or external refs. Apifox: Settings → Export data → Postman. ApiPost: Project settings → Export project → Postman 2.1. Their proprietary project formats are unsupported.",
	Format: "Detected format: %s", Destination: "Destination: %s", Counts: "Requests %d · Folders %d · Variables %d",
	NewRequest: "New unsaved request", NewEntity: "Creates a new entity without overwriting existing data. Activate environments manually.",
	NativeCollection: "Reva v1 collection", NativeEnvironment: "Reva v1 environment", PostmanCollection: "Postman collection v2.0/v2.1",
	PostmanEnvironment: "Postman environment", HAR: "HAR 1.2 request collection", Curl: "curl request",
	Unknown: "Unrecognized content. Choose a supported format.", InvalidCurl: "Invalid curl command or unsupported option. Check quoting, URL, and arguments.",
	InvalidPostman: "Invalid Postman collection. Check the JSON and request structure.", FileError: "Cannot read this file. Choose a readable file.",
	SaveError: "Cannot save the import. Check the storage location, then retry.", Success: "Collection imported", EnvironmentSuccess: "Environment imported", RequestSuccess: "Request opened", Loading: "Reading file…",
	InvalidJSON: "Invalid JSON. Check encoding, duplicate fields, and document completeness.", UnsupportedFormat: "Unsupported format. Export Apifox / ApiPost as Postman first; Insomnia, bulk backups, and YAML are unsupported.",
	UnsupportedVersion: "Unsupported version. Use Swagger 2.0, OpenAPI 3.0/3.1, Reva v1, Postman v2.0/v2.1, or HAR 1.2.", AmbiguousFormat: "Content contains conflicting format markers. Import one collection or environment at a time.",
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
	case "invalid_source_url":
		return s.InvalidSourceURL
	case "fetch_failed":
		return s.FetchFailed
	case "unsupported_reference":
		return s.UnsupportedReference
	case "unsupported_parameter":
		return s.UnsupportedParameter
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
	case "openapi_review":
		return s.OpenAPIReview
	case "openapi_selection":
		return s.OpenAPISelection
	case "openapi_empty_server":
		return s.OpenAPIEmptyServer
	case "openapi_schema_choice":
		return s.OpenAPISchemaChoice
	case "openapi_schema_incomplete":
		return s.OpenAPISchemaIncomplete
	case "openapi_nonstandard_path":
		return s.OpenAPINonstandardPath
	}
	return s.MetadataOmitted
}
