package i18n

// Collections holds the strings of the collections sidebar and save dialog. Owned by that feature's PR.
type Collections struct {
	Settings, FolderSettings, SettingsTitle, FolderSettingsTitle             string
	General, DiscardReload, Unsaved, SettingsSaved, SettingsSaveFailed       string
	SettingsMissing, SettingsHelp, SettingsNavigation, SettingsContent       string
	DiscardHelp, SettingsFor, SettingsForTarget, RequestCount, SidebarCount  string
	APISection, OrganizeSection, ExportSection, NativeJSON, NativeJSONHelp   string
	GuideSection, GuideIntro, GuideScopeTitle, GuideScope, GuideFolderScope  string
	GuideVariablesTitle, GuideVariables, GuideAuthTitle, GuideAuth           string
	GuideEditingTitle, GuideEditing                                          string
	RunCollection, RunStart, RunCancel, RunClose, RunSetup                   string
	RunEnvironment, RunNoEnvironment, RunStopOnFailure                       string
	RunRunning, RunCancelling, RunMissingCollection, RunMissingEnvironment   string
	RunPassed, RunFailed, RunCancelled, RunSkipped, RunCounts, RunAssertions string
	RunRows, RunOrdinal, RunStatus, RunOutcome, RunAssertionColumn           string
	RunNoStatus, RunEmpty                                                    string

	Authentication, AuthSaved, AuthSaveFailed, AuthTargetMissing string
	Title, NewCollection, NewFolder, NewRequest                  string
	NewGraphQL, NewWebSocket                                     string
	Empty, Name, Collection, Folder, Root                        string
	SaveRequest, Save, Saved, Cancel                             string
	Rename, Delete, DeleteTitle, DeletePrompt                    string
	Open, Duplicate, CopyName, Create                            string
	InlineCollection, CollectionName                             string
	Add, Actions, Filter, NoMatches                              string
	Variables, Export, VariablesSaved, Exported                  string
}

var collectionsZh = Collections{
	Settings: "集合设置", FolderSettings: "文件夹设置", SettingsTitle: "%s · 集合设置", FolderSettingsTitle: "%s · 文件夹设置",
	General: "概览", DiscardReload: "放弃", DiscardHelp: "放弃更改并重新加载", Unsaved: "有未保存的更改", SettingsSaved: "设置已保存", SettingsSaveFailed: "无法保存设置，请重试。",
	SettingsFor: "%s 的设置", SettingsForTarget: "%s 的设置（%s）", RequestCount: "共 %d 个请求", SidebarCount: "(%d)",
	APISection: "API", OrganizeSection: "组织", ExportSection: "导出", NativeJSON: "Reva JSON 文件", NativeJSONHelp: "导出已保存的集合，包含变量、共享认证和请求。",
	GuideSection: "集合指南", GuideIntro: "在同一工作区中组织 API、配置共享变量与认证。此指南说明配置的作用范围及保存方式。",
	GuideScopeTitle: "集合概览", GuideScope: "左侧统计包含集合及所有子文件夹中已保存的请求。HTTP 包含 GraphQL 请求；空文件夹不计入请求总数。使用加号在当前集合中创建请求。",
	GuideFolderScope:    "左侧统计仅包含当前文件夹及其子文件夹中已保存的请求。HTTP 包含 GraphQL 请求；空文件夹不计入总数。使用加号在当前文件夹中创建请求。",
	GuideVariablesTitle: "共享变量", GuideVariables: "在“集合变量”中维护键值。请求可通过 {{变量名}} 引用；启用的环境变量会覆盖同名集合变量。文件夹使用所属集合的变量。",
	GuideAuthTitle: "认证与继承", GuideAuth: "集合和文件夹可以配置共享认证。设为继承的 HTTP 请求使用最近的文件夹或集合认证；“无认证”会停止继承。WebSocket 使用手动请求头。",
	GuideEditingTitle: "编辑与保存", GuideEditing: "名称、变量和认证的修改会保留在当前标签页草稿中。切换分区不会丢失草稿；点击“保存”后才对请求生效。“放弃”会重新加载已保存的值。导出使用已保存的数据，可能包含凭据。",
	SettingsMissing: "集合或文件夹已不存在。无法保存，请关闭此标签页。", SettingsHelp: "在此管理名称、共享认证和集合变量。更改仅在保存后生效。", SettingsNavigation: "设置导航", SettingsContent: "配置内容",
	RunCollection: "运行集合", RunStart: "开始运行", RunCancel: "取消运行", RunClose: "关闭",
	RunSetup:       "按集合中的顺序逐个运行已保存的请求。未保存的更改不会参与运行。",
	RunEnvironment: "运行环境", RunNoEnvironment: "不使用环境", RunStopOnFailure: "遇到失败时停止",
	RunRunning: "正在运行集合…", RunCancelling: "正在取消运行…",
	RunMissingCollection: "此集合已不存在。", RunMissingEnvironment: "所选环境已不存在，请选择其他环境或不使用环境。",
	RunPassed: "通过", RunFailed: "失败", RunCancelled: "已取消", RunSkipped: "已跳过",
	RunCounts: "请求共 %d 项 · 通过 %d · 失败 %d · 取消 %d · 跳过 %d", RunAssertions: "断言：通过 %d · 失败 %d",
	RunRows: "请求结果（按集合顺序）", RunOrdinal: "序号", RunStatus: "HTTP 状态", RunOutcome: "结果", RunAssertionColumn: "断言", RunNoStatus: "—", RunEmpty: "集合中没有已保存的请求。",

	Authentication: "身份认证", AuthSaved: "认证已保存", AuthSaveFailed: "无法保存认证，请重试。", AuthTargetMissing: "集合或文件夹已不存在，无法保存认证。",
	NewWebSocket: "新建 WebSocket",
	NewGraphQL:   "新建 GraphQL 请求",
	Title:        "我的集合", NewCollection: "新建集合", NewFolder: "新建文件夹", NewRequest: "新建请求",
	Empty: "尚无集合，创建集合以保存请求。", Name: "名称", Collection: "集合", Folder: "文件夹", Root: "集合根目录",
	SaveRequest: "保存请求", Save: "保存", Saved: "请求已保存", Cancel: "取消",
	Rename: "重命名", Delete: "删除", DeleteTitle: "确认删除", DeletePrompt: "删除“%s”及其所有内容？",
	Open: "打开", Duplicate: "创建副本", CopyName: "%s 副本", Create: "创建",
	InlineCollection: "创建新集合", CollectionName: "集合名称",
	Add: "添加集合项", Actions: "%s 的操作", Filter: "筛选集合与请求", NoMatches: "没有匹配的请求",
	Variables: "集合变量", Export: "导出集合",
	VariablesSaved: "集合变量已保存", Exported: "集合已导出",
}

var collectionsEn = Collections{
	Settings: "Collection settings", FolderSettings: "Folder settings", SettingsTitle: "%s · Collection settings", FolderSettingsTitle: "%s · Folder settings",
	General: "Overview", DiscardReload: "Discard", DiscardHelp: "Discard changes and reload", Unsaved: "Unsaved changes", SettingsSaved: "Settings saved", SettingsSaveFailed: "Could not save settings. Please try again.",
	SettingsFor: "Settings for %s", SettingsForTarget: "Settings for %s (%s)", RequestCount: "%d requests", SidebarCount: "(%d)",
	APISection: "API", OrganizeSection: "Organize", ExportSection: "Export", NativeJSON: "Reva JSON file", NativeJSONHelp: "Export the saved collection, including variables, shared authentication and requests.",
	GuideSection: "Collection guide", GuideIntro: "Organize APIs and configure shared variables and authentication in one workspace. This guide explains their scope and how changes are saved.",
	GuideScopeTitle: "Collection overview", GuideScope: "The API counts include saved requests in this collection and all its folders. HTTP includes GraphQL requests; empty folders do not count as requests. Use the plus buttons to create requests in this collection.",
	GuideFolderScope:    "The API counts include saved requests in this folder and its descendants only. HTTP includes GraphQL requests; empty folders do not count. Use the plus buttons to create requests in this folder.",
	GuideVariablesTitle: "Shared variables", GuideVariables: "Maintain key/value pairs in Collection variables. Requests can reference them with {{name}}; enabled environment variables override collection values with the same name. Folders use their collection's variables.",
	GuideAuthTitle: "Authentication and inheritance", GuideAuth: "Collections and folders can configure shared authentication. HTTP requests set to inherit use the nearest folder or collection configuration; None stops inheritance. WebSocket uses manual headers.",
	GuideEditingTitle: "Editing and saving", GuideEditing: "Name, variable and authentication edits stay in this tab's draft when you switch sections. Save applies them to requests; Discard reloads saved values. Export uses saved data and can include credentials.",
	SettingsMissing: "The collection or folder no longer exists. Close this tab; its changes cannot be saved.", SettingsHelp: "Manage names, shared authentication and collection variables here. Changes take effect only when saved.", SettingsNavigation: "Settings navigation", SettingsContent: "Configuration content",
	RunCollection: "Run collection", RunStart: "Start run", RunCancel: "Cancel run", RunClose: "Close",
	RunSetup:       "Run the saved requests one at a time in collection order. Unsaved changes are excluded.",
	RunEnvironment: "Run environment", RunNoEnvironment: "No environment", RunStopOnFailure: "Stop on failure",
	RunRunning: "Running collection…", RunCancelling: "Cancelling run…",
	RunMissingCollection: "This collection is no longer available.", RunMissingEnvironment: "The selected environment is no longer available. Choose another or select No environment.",
	RunPassed: "Passed", RunFailed: "Failed", RunCancelled: "Cancelled", RunSkipped: "Skipped",
	RunCounts: "%d requests · %d passed · %d failed · %d cancelled · %d skipped", RunAssertions: "Assertions: %d passed · %d failed",
	RunRows: "Request results in collection order", RunOrdinal: "Ordinal", RunStatus: "HTTP status", RunOutcome: "Outcome", RunAssertionColumn: "Assertions", RunNoStatus: "—", RunEmpty: "The collection has no saved requests.",

	Authentication: "Authentication", AuthSaved: "Authentication saved", AuthSaveFailed: "Could not save authentication. Please try again.", AuthTargetMissing: "The collection or folder no longer exists. Authentication could not be saved.",
	NewWebSocket: "New WebSocket",
	NewGraphQL:   "New GraphQL request",
	Title:        "My collections", NewCollection: "New collection", NewFolder: "New folder", NewRequest: "New request",
	Empty: "No collections yet. Create one to save requests.", Name: "Name", Collection: "Collection", Folder: "Folder", Root: "Collection root",
	SaveRequest: "Save request", Save: "Save", Saved: "Request saved", Cancel: "Cancel",
	Rename: "Rename", Delete: "Delete", DeleteTitle: "Confirm deletion", DeletePrompt: "Delete “%s” and all its contents?",
	Open: "Open", Duplicate: "Duplicate", CopyName: "%s copy", Create: "Create",
	InlineCollection: "Create a new collection", CollectionName: "Collection name",
	Add: "Create collection item", Actions: "Actions for %s", Filter: "Filter collections and requests", NoMatches: "No matching requests",
	Variables: "Collection variables", Export: "Export collection",
	VariablesSaved: "Collection variables saved", Exported: "Collection exported",
}
