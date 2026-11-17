package i18n

// Collections holds the strings of the collections sidebar and save dialog. Owned by that feature's PR.
type Collections struct {
	Settings, FolderSettings, SettingsTitle, FolderSettingsTitle       string
	General, DiscardReload, Unsaved, SettingsSaved, SettingsSaveFailed string
	SettingsMissing, SettingsHelp, SettingsNavigation, SettingsContent string

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
	General: "常规", DiscardReload: "放弃更改并重新加载", Unsaved: "有未保存的更改", SettingsSaved: "设置已保存", SettingsSaveFailed: "无法保存设置，请重试。",
	SettingsMissing: "集合或文件夹已不存在。无法保存，请关闭此标签页。", SettingsHelp: "在此管理名称、共享认证和集合变量。更改仅在保存后生效。", SettingsNavigation: "设置导航", SettingsContent: "配置内容",

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
	General: "General", DiscardReload: "Discard and reload", Unsaved: "Unsaved changes", SettingsSaved: "Settings saved", SettingsSaveFailed: "Could not save settings. Please try again.",
	SettingsMissing: "The collection or folder no longer exists. Close this tab; its changes cannot be saved.", SettingsHelp: "Manage names, shared authentication and collection variables here. Changes take effect only when saved.", SettingsNavigation: "Settings navigation", SettingsContent: "Configuration content",

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
