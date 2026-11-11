package i18n

// Env holds the strings of the environment selector and editor. Owned by that feature's PR.
type Env struct {
	Title, NoEnvironment, Edit, New, NewEnvironment string
	Rename, Duplicate, CopyName, Delete             string
	Name, Enabled, Key, Value, DeleteRow, AddRow    string
	Save, Close, Empty, SaveHint, NameRequired      string
	Unresolved, UnresolvedTip, Error                string
}

var envZh = Env{
	Title: "环境", NoEnvironment: "无环境", Edit: "编辑环境", New: "新建环境", NewEnvironment: "新环境",
	Rename: "重命名", Duplicate: "复制环境", CopyName: "%s（副本）", Delete: "删除环境",
	Name: "环境名称", Enabled: "启用", Key: "变量名", Value: "变量值", DeleteRow: "删除变量", AddRow: "添加变量",
	Save: "保存环境", Close: "关闭", Empty: "新建或选择一个环境以编辑变量。",
	SaveHint: "保存会应用所有环境的更改；关闭会丢弃未保存的更改。", NameRequired: "请输入环境名称。",
	Unresolved: "%d 个未解析变量", UnresolvedTip: "未解析的变量：", Error: "环境操作失败",
}

var envEn = Env{
	Title: "Environments", NoEnvironment: "No environment", Edit: "Edit environments", New: "New environment", NewEnvironment: "New environment",
	Rename: "Rename", Duplicate: "Duplicate environment", CopyName: "%s (copy)", Delete: "Delete environment",
	Name: "Environment name", Enabled: "Enabled", Key: "Variable key", Value: "Variable value", DeleteRow: "Delete row", AddRow: "Add variable",
	Save: "Save environments", Close: "Close", Empty: "Create or select an environment to edit its variables.",
	SaveHint: "Save applies changes to all environments; closing discards unsaved changes.", NameRequired: "Enter an environment name.",
	Unresolved: "%d unresolved variables", UnresolvedTip: "Unresolved variables: ", Error: "Environment operation failed",
}
