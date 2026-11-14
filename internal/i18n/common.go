package i18n

// Common holds strings shared by every feature. Owned by the foundation;
// add to it only when two or more features need the same word.
type Common struct {
	AppName string
	OK      string
	Cancel  string
	Save    string
	Delete  string
	Rename  string
	Close   string
	Name    string
	Key     string
	Value   string
	Add     string
	Error   string
}

var commonZh = Common{
	AppName: "Reva",
	OK:      "确定",
	Cancel:  "取消",
	Save:    "保存",
	Delete:  "删除",
	Rename:  "重命名",
	Close:   "关闭",
	Name:    "名称",
	Key:     "键",
	Value:   "值",
	Add:     "添加",
	Error:   "错误",
}

var commonEn = Common{
	AppName: "Reva",
	OK:      "OK",
	Cancel:  "Cancel",
	Save:    "Save",
	Delete:  "Delete",
	Rename:  "Rename",
	Close:   "Close",
	Name:    "Name",
	Key:     "Key",
	Value:   "Value",
	Add:     "Add",
	Error:   "Error",
}
