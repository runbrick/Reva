package i18n

// Env holds the environment selector, sidebar and editor strings.
type Env struct {
	Title, NoEnvironment, Edit, Manage, New, NewEnvironment     string
	Rename, Duplicate, CopyName, Delete, DeleteConfirm          string
	Name, Enabled, Key, Value, Description, DeleteRow, AddRow   string
	Save, Close, Discard, Cancel, Empty, SaveHint, NameRequired string
	SetActive, Actions, Unsaved, UnsavedMark                    string
	Unresolved, UnresolvedTip, Error                            string
}

var envZh = Env{
	Title: "环境", NoEnvironment: "无环境", Edit: "编辑环境", Manage: "管理环境", New: "新建环境", NewEnvironment: "新环境",
	Rename: "重命名", Duplicate: "复制环境", CopyName: "%s（副本）", Delete: "删除环境", DeleteConfirm: "确定删除环境「%s」及其所有变量？此操作无法撤销。",
	Name: "环境名称", Enabled: "启用", Key: "变量名", Value: "变量值", Description: "变量描述", DeleteRow: "删除变量", AddRow: "添加变量",
	Save: "保存", Close: "关闭", Discard: "放弃更改", Cancel: "取消", Empty: "新建或选择一个环境以编辑变量。",
	SaveHint: "保存会应用当前环境的更改；放弃更改会恢复上次保存的内容。", NameRequired: "请输入环境名称。",
	SetActive: "设为活动环境", Actions: "环境操作", Unsaved: "未保存的更改", UnsavedMark: "*",
	Unresolved: "%d 个未解析变量", UnresolvedTip: "未解析的变量：", Error: "环境操作失败",
}

var envEn = Env{
	Title: "Environments", NoEnvironment: "No environment", Edit: "Edit environments", Manage: "Manage environments", New: "New environment", NewEnvironment: "New environment",
	Rename: "Rename", Duplicate: "Duplicate environment", CopyName: "%s (copy)", Delete: "Delete environment", DeleteConfirm: "Delete environment “%s” and all its variables? This cannot be undone.",
	Name: "Environment name", Enabled: "Enabled", Key: "Variable key", Value: "Variable value", Description: "Variable descriptions", DeleteRow: "Delete row", AddRow: "Add variable",
	Save: "Save", Close: "Close", Discard: "Discard changes", Cancel: "Cancel", Empty: "Create or select an environment to edit its variables.",
	SaveHint: "Save applies changes to this environment; Discard restores the last saved values.", NameRequired: "Enter an environment name.",
	SetActive: "Set active", Actions: "Environment actions", Unsaved: "Unsaved changes", UnsavedMark: "*",
	Unresolved: "%d unresolved variables", UnresolvedTip: "Unresolved variables: ", Error: "Environment operation failed",
}
