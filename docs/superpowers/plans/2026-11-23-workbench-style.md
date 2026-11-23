# Reva 工作台样式优化实施计划

**目标**　依据 `docs/design/target.png`，让发送、当前导航和当前标签页易于定位，并提高浅色和深色主题下表格、状态及代码的可读性。

**实现范围**　由 `antigravity/gemini-4-neon` 调整现有 Viva 主题和工作台导航。主题令牌集中在 `internal/gui/themes`，继续由现有界面使用。

## 文件范围

- `internal/gui/themes/studio.go` 集中定义蓝色强调色、表面层级、选中态及语义色。
- `internal/gui/themes/themes_test.go` 验证主要操作与前景、选中态文字、状态色和代码颜色的对比度，保留系统主题与布局指标的兼容性检查。
- `internal/gui/studio.go` 使用主题令牌突出当前侧栏入口，保留现有交互及控件尺寸。

## 实施步骤

- [x] 以现有无头截图和参考图为基准，确定浅色背景与紧凑深色工作台的视觉层级。
- [x] 为两套主题添加 `TestThemeHierarchyAndContrast`。运行 `go test ./internal/gui/themes -count=1`，确认新断言能捕获选中行的方法徽章对比度不足。
- [x] 调整 `Resolve(dark bool) (ui.Theme, Palette)` 和 `studioRailButton` 的样式。将颜色定义放在主题中，沿用现有间距、字号与 Lucide 图标。
- [x] 重跑主题测试，并用 `TestStudioHeadlessCapture` 检查宽版浅色工作台、窄版深色工作台、设置和环境页。用现有测试验证中英文与系统主题切换。
- [x] 运行 `gofmt -l .`、`go vet ./...`、`go tool viva vet ./...` 和 `go test ./... -count=1`，核对差异后提交当前分支。

## 验收重点

- 发送按钮文字、常用状态与代码文字在实际底色上达到 4.5:1 的对比度。
- 导航选中态有独立背景，主要操作和当前标签页使用蓝色强调，HTTP 方法与响应状态保有各自语义色。
- 系统主题切换和中英文界面沿用现有行为，宽窄窗口中的主要控件保持可见。
