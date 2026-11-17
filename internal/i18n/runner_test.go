package i18n_test

import (
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
)

func TestRunnerHelpExplainsBehaviorInBothLanguages(t *testing.T) {
	for _, tc := range []struct {
		lang i18n.Lang
		want []string
	}{
		{i18n.Zh, []string{"用法：", "默认不选择环境", "仅读取现有数据", "不保存设置", "上传文件", "输入错误不生成报告", "退出码：", "编译后的程序"}},
		{i18n.En, []string{"Usage:", "No environment is selected by default", "Reads existing data only", "never saves settings", "upload files", "Input errors produce no report", "Exit codes:", "built binary"}},
	} {
		t.Run(string(tc.lang), func(t *testing.T) {
			help := i18n.RunnerFor(tc.lang).Help
			for _, want := range append(tc.want, "--collection ID", "--data-dir DIR", "--environment ID", "--stop-on-failure", "-h", "--help", "REVA_HOME", "JSON", "0", "1", "2", "130") {
				if !strings.Contains(help, want) {
					t.Errorf("help must explain %q", want)
				}
			}
		})
	}
	if i18n.RunnerFor(i18n.Zh).Help == i18n.RunnerFor(i18n.En).Help {
		t.Fatal("Chinese and English help must each be localized")
	}
	if i18n.RunnerFor("unknown") != i18n.RunnerFor(i18n.Zh) {
		t.Fatal("unsupported languages must fall back to Chinese")
	}
}
