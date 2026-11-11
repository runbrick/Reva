package i18n

// Import holds the strings of the import dialog. Owned by that feature's PR.
type Import struct {
	Title, Description, Input, Placeholder string
	LoadFile, JSONFiles, Submit, Cancel    string
	Unknown, InvalidCurl, InvalidPostman   string
	FileError, SaveError, Success, Loading string
}

var importZh = Import{
	Title: "导入", Description: "粘贴 curl 命令或 Postman Collection v2.0 / v2.1 JSON，也可打开 JSON 文件。",
	Input: "导入内容", Placeholder: "粘贴 curl 命令或 Postman JSON…",
	LoadFile: "打开 JSON 文件", JSONFiles: "JSON 文件", Submit: "开始导入", Cancel: "取消",
	Unknown:        "无法识别内容，请粘贴 curl 命令或 Postman 集合 JSON。",
	InvalidCurl:    "curl 命令无效或包含不支持的选项，请检查引号、URL 和参数。",
	InvalidPostman: "Postman 集合无效，请检查 JSON、集合名称和请求列表。",
	FileError:      "无法读取文件，请选择可读取的 JSON 文件。", SaveError: "无法保存集合，请重试。",
	Success: "集合导入成功", Loading: "正在读取文件…",
}

var importEn = Import{
	Title: "Import", Description: "Paste a curl command or Postman Collection v2.0 / v2.1 JSON, or open a JSON file.",
	Input: "Import content", Placeholder: "Paste curl or Postman JSON…",
	LoadFile: "Open JSON file", JSONFiles: "JSON files", Submit: "Start import", Cancel: "Cancel",
	Unknown:        "Unrecognized content. Paste a curl command or Postman collection JSON.",
	InvalidCurl:    "Invalid curl command or unsupported option. Check quoting, URL, and arguments.",
	InvalidPostman: "Invalid Postman collection. Check the JSON, collection name, and item array.",
	FileError:      "Cannot read this file. Choose a readable JSON file.", SaveError: "Cannot save the collection. Please try again.",
	Success: "Collection imported", Loading: "Reading file…",
}
