# MCP 工具缺口补齐设计（MCP Tools Gap-Fill Design）

- 基础设计：`docs/mcp-tools.md`（适配层目标、非目标、分层、工具契约）
- 遵循：`docs/ENGINEERING_PRINCIPLES.md` 全部条款；冲突时以本文件与 ENGINEERING_PRINCIPLES 为准
- 关联文档：`API_CN.md`（公共 API 契约，本设计的映射依据）

## 1. 背景与一句话目标

`internal/mcpadapter` 现有 14 个 MCP tools，全部依赖 `target_id`（窗口），只覆盖了公共 API 的"窗口绑定输入"子集。本设计补齐对外部 agent 而言存在的公共 API 能力缺口，让 MCP 层可脱离窗口执行基础桌面输入，并修正已有工具的确定性缺陷。

一句话目标：**在不改动核心库公共 API 的前提下，为 MCP 适配层补齐"屏幕坐标全局输入"与"缺口修复"，且全部映射到 `winput`/`screen` 既有导出函数。**

## 2. 现状能力矩阵（审计结果）

对照 `API_CN.md` 与代码导出，公共 API → MCP tools 覆盖情况：

| 公共 API | MCP tool | 状态 |
|---|---|---|
| `FindByTitle/FindByClass/FindByProcessName` | `find_window` | 已覆盖，多结果静默取第一个 |
| `FindByPID` | — | **缺口** |
| `FindChildByClass` | `find_child` | 已覆盖 |
| `Window.Move / Click / Right / Middle / DoubleClick` | `move_mouse / click / right_click / double_click` | 已覆盖 |
| `Window.Type / Press / PressHotkey` | `type_text / press_key / press_hotkey` | 已覆盖，`press_key`/`press_hotkey` 仅支持按键子集 |
| `Window.Text / Value` | `read_text`（`best_effort`） | 已覆盖 |
| `Window.Scroll` | — | **缺口**（无定向滚动） |
| `Window.MoveRel` | — | 缺口（以后做） |
| `Window.ScreenToClient / ClientToScreen` | — | 缺口（以后做，随图像回传流） |
| 全局 `MoveMouseTo / ClickMouseAt / Right / Middle / DoubleClick / ScrollWheel` | — | **缺口**（无屏幕坐标输入族） |
| 全局 `KeyDown / KeyUp / Press / PressHotkey / Type / PressFor` | — | 缺口（以后做） |
| 全局 `MouseDown / MouseUp / MoveMouseBy / MoveMouseToExact` | — | 缺口（以后做） |
| `SetBackend` | `switch_backend` | 已覆盖 |
| `GetCursorPos` / `screen.VirtualBounds` | `get_cursor_pos` / `get_virtual_bounds` | 已覆盖 |
| `screen.CaptureVirtualDesktop` | `capture_screen` | 已覆盖但默认进程必失败（DPI 前置未满足，见 §4.1） |
| `screen.CaptureRegion / Monitors / ImageToVirtual` | — | 缺口（以后做，随图像回传流） |
| `Window.DPI / ClientRect / IsValid / IsVisible` | find_window/child 元数据 | 已覆盖 |

最大断层：**屏幕坐标/全局输入整族缺失**——`capture_screen`/`get_cursor_pos`/`get_virtual_bounds` 返回的坐标没有任何 tool 能消费，"定位→点击"闭环在 MCP 层不可用。

## 3. 范围矩阵

| | 内容 |
|---|---|
| **现在做** | ① `capture_screen` DPI 前置修复（§4.1）；② 屏幕坐标全局输入族 `move_mouse_to` / `click_at` / `double_click_at` / `scroll_wheel`（§4.2–4.5）；③ 窗口定向滚动 `scroll`（§4.6）；④ `find_window` 增强 `pid` + `index` + `count` 消歧（§4.7）；⑤ 按键契约扩展（§4.8） |
| **以后做** | 截图图像回传 / `capture_region`、全局键盘族、`MouseDown`/`MouseUp`/`MoveMouseBy`/`MoveMouseToExact`、`ScreenToClient`/`ClientToScreen`、`screen.Monitors`/`ImageToVirtual`、`Window.MoveRel`、`list_windows` |
| **永不做** | OCR/CV/DOM 自动化、agent runtime、技能/规划逻辑（承袭 `docs/mcp-tools.md` 非目标）；HWND 等 provider 原生句柄进入 MCP 契约；为核心库增加 MCP 专用接口 |

现在做判据：直接映射既有导出函数、无新抽象、有明确可验收产出，且修复真实矫正性问题（静默取首个窗口、默认进程 capture 必失败、按键子集过窄）。

## 4. 工具设计

公共契约（所有新增工具沿用 `docs/mcp-tools.md` §Tool Contract）：`name`/`description`/`inputSchema`/`outputSchema`/`errorSchema`/`annotations`；错误走 `toJSONRPCError` 的 `code/retryable/details` 结构；`state_change` 受 `--allow-mutations` 门控。

### 4.1 capture_screen DPI 前置修复（缺陷修复）

- **问题**：`screen.CaptureVirtualDesktop` 在 `!window.IsPerMonitorDPIAware()` 时直接返回错误；`cmd/mcp-server/main.go` 未调用 `EnablePerMonitorDPI()`，默认进程下 `capture_screen` 必失败。
- **设计**：`Server` 初始化时调用一次 `winput.EnablePerMonitorDPI()`；失败记结构化告警（`log.Printf`），不使 server 崩溃；`capture_screen` 保持"仅返回 width/height"的既有输出契约（图像回传属以后做）。
- **映射**：`winput.EnablePerMonitorDPI()` → server 生命周期，非新 tool。
- **验收**：默认进程（无任何 DPI 设置）启动 server，调用 `capture_screen` 返回非零 `width`/`height`，无 `not Per-Monitor DPI Aware` 错误。

### 4.2 `move_mouse_to(x, y)`（新工具）

- **映射**：`winput.MoveMouseTo(x, y)`。
- **入参**：`x`/`y`（integer，虚拟桌面绝对坐标，与 `get_cursor_pos`/`get_virtual_bounds` 同坐标系）。
- **出参**：`okSchema`。
- **元数据**：`annotations("state_change", true)`。
- **取舍**：映射 `MoveMouseTo` 而非 `MoveMouseToExact`（后者含 12ms 等待与收敛校验，语义更重；作为以后做选项）。`MoveMouseTo` 实现与 `MoveMouseToExact` 的边界差异以代码为准。
- **验收**：schema 断言；`tools/call` 冒烟（真实桌面人工）。函数映射本身无不可测纯逻辑。

### 4.3 `click_at(x, y, button)`（新工具）

- **映射**：`button` 为 `"left"|"right"|"middle"`（默认 `left`）→ `winput.ClickMouseAt` / `ClickRightMouseAt` / `ClickMiddleMouseAt`；非法 button → `validation_error`。
- **入参**：`x`/`y`（integer 必填）、`button`（stringEnum）。
- **出参**：`okSchema`。
- **元数据**：`annotations("state_change", true)`。
- **验收**：button 分派表驱动单测（纯函数 `clickAtButton` 抽取）；schema 断言。

### 4.4 `double_click_at(x, y)`（新工具）

- **映射**：`winput.DoubleClickMouseAt(x, y)`。
- **入参**：`x`/`y`（integer 必填）。
- **出参**：`okSchema`。
- **元数据**：`annotations("state_change", true)`。
- **确定事项**：与窗口族 `double_click` 保持同名异受众的对称；不做"click_at 里加 double 布尔"（与现有目录粒度一致，避免单一工具承载双语义）。

### 4.5 `scroll_wheel(steps)`（新工具）

- **映射**：`winput.ScrollWheel(steps int)`（整格滚动，正前负后；`0` 或 `\|steps\|>273` 由库返回 `ErrInvalidScrollSteps`）。
- **入参**：`steps`（integer 必填）。
- **出参**：`okSchema`。
- **错误**：`ErrInvalidScrollSteps` → `validation_error`（调用方输入问题）；其余沿用 `mapLibraryError`。
- **元数据**：`annotations("state_change", true)`。
- **验收**：非法 `steps` 返回 `validation_error`；schema 断言。

### 4.6 `scroll(target_id, x, y, delta)`（新工具，窗口定向滚动）

- **映射**：`Window.Scroll(x, y, delta)`。
- **入参**：`target_id`、`x`、`y`、`delta`（integer 必填）。
- **出参**：`okSchema`。
- **错误**：`target_id` 无效 → `target_not_found`（既有）；`ErrDirectedScrollUnsupported`（HID 后端无法定向到窗口）→ `backend_unavailable`（该能力当前后端不支持，操作未执行，硬告警）。
- **元数据**：`annotations("state_change", true)`；description 注明 message 后端可用、HID 返回 `backend_unavailable`。
- **验收**：HID 分支错误映射单测；schema 断言。

### 4.7 find_window 增强：`pid` + `index` + `count`

- **新增可选入参** `pid`（integer）→ `winput.FindByPID(pid)` 候选列表。
- **新增可选入参** `index`（integer，默认 0）：从候选列表中选第 N 个；越界 → `validation_error`。
- **新增输出字段** `count`（integer）：命中候选总数（消歧信号，不改变现有字段）。
- **兼容**：`title`/`class`（单返回语义）不变；`process_name`/`pid` 走候选列表 + `index`；所有既有字段、类型、必填集不变——纯 additive（对应 `docs/mcp-tools.md` §Compatibility Strategy）。
- **候选选择逻辑抽取为纯函数** `pickCandidate(list, index)`，便于单元测试（真实 Win32 调用不进 CI）。
- **验收**：候选选择与 `index` 越界表驱动单测；`find_window` schema 含 `pid`/`index`/`count` 断言。
- **确定事项**：不做 `list_windows`（本设计以后做），`index`+`count` 已解决静默取首的矫正性问题；`list_windows` 价值被稀释，避免过度设计。

### 4.8 按键契约扩展（press_key / press_hotkey）

- **现状**：`keyFromName` 仅支持 A–Z、ENTER/TAB/ESC/SPACE/CTRL/SHIFT/ALT、四方向键；库 `Key` 常量含 F1–F12、0–9、`Insert/Delete/Home/End/PageUp/PageDown/Caps`、标点，均无法经 MCP 表达。
- **设计**：扩展 `keyFromName`：① 名称表补齐全部 `Key` 常量名（含 F1–F12、0–9、Insert/Delete/Home/End/PageUp/PageDown、CapsLock）；② 单个字符直接经 `winput.KeyFromRune`（覆盖 ",", ".", ";", "-" 等标点，返回 `ok` 标识）；不支持/无法解析 → `validation_error`。
- **schema**：`press_key` 的 `key` 与 `press_hotkey` 的 `keys[]` 保持 string 描述（不改为巨型枚举——与库解析器职责重复）。
- **资源上限**（对应 ENGINEERING §7）：`press_hotkey` 的 `keys` 数组补 `maxItems: 8`，防超长输入。
- **验收**：表驱动单测——新增合法名 → 正确 `Key`；单字符 → 经 `KeyFromRune`；非法 → `validation_error`；`tools/list` schema 含 `maxItems` 断言。

## 5. 分层与铁律

- MCP 适配层只作为 `winput`/`screen` 的**消费者**，调用既有导出函数；不为任何新 tool 修改核心库公共 API（承袭 `docs/mcp-tools.md` §Layering 与 Public Boundary Rules）。
- 新增工具采用 additive 演进：不加必填字段、不删不改既有字段类型（§4.7 明确列出）。
- `target_id` 不透明、进程内有效；不承诺跨进程/跨重启存活（承袭）。
- HWND 等 provider 原生句柄不得进入 MCP 契约（承袭）。

## 6. 失败政策

- 单个 tool 调用失败 → 结构化错误（`code`/`retryable`/`details`），不中断 server（既有 `toJSONRPCError`）。
- 新增映射：`ErrInvalidScrollSteps` → `validation_error`；`ErrDirectedScrollUnsupported` → `backend_unavailable`（操作未执行，硬告警）。
- `capture_screen` DPI 初始化失败：记告警，server 继续运行；capture 调用错误按库返回透传。
- 空/无候选中窗口 → `target_not_found`（既有），不当作成功。

## 7. 已知限制

- `capture_screen` 仅返回尺寸、位图被丢弃（沿用现有契约）；图像回传、区域截图属以后做。
- 屏幕坐标输入族的坐标均指虚拟桌面绝对坐标；client 坐标换算工具（`ClientToScreen`/`ScreenToClient`）属以后做，窗口客户区坐标由调用方自行换算。
- 全局键盘族（无窗口 `Press`/`Type`/`PressFor`）属以后做；当前键盘工具均要求 `target_id`。

## 8. 验证与验收

| 项目 | 方法 | 通过标准 |
|---|---|---|
| 纯逻辑（候选选择、按键映射、button 分派、index 越界、maxItems） | 表驱动单元测试（`internal/mcpadapter`，沿用现有 `server_test.go` 风格） | 全部通过 |
| 新工具 schema（input/output/annotations/required/errorSchema） | `tools/list` 结构断言（沿用 `TestToolsListIncludesSchemas` 模式） | 全部通过 |
| 变更门控 | 默认（无 `--allow-mutations`）下调用 `state_change` 工具 → `unsafe_operation` | 断言通过 |
| DPI 前置（§4.1） | 默认进程启动 server 调 `capture_screen` | 返回非零尺寸，无 DPI 错误 |
| 真实输入 | 真实桌面人工冒烟（不进 CI） | click/scroll/按键生效 |

## 9. 反过度设计复核

- 屏幕输入族采用与窗口族一致的单工具粒度（`click_at`/`double_click_at`/`scroll_wheel`/`move_mouse_to`），拒绝"一个 `mouse` 工具挂 5 个入参开关"的设计——后者让契约膨胀且与现有目录不一致。
- `find_window` 用 `index`+`count` 而非新建 `list_windows`，消歧成本最小。
- 全局键盘、坐标换算、`MouseDown`/`Up` 等未进入现在做：窗口族已覆盖主流场景，其余依赖图像回传流才能体现价值，属以后做，避免现在引入无法验收的抽象。
- 按键扩展复用库的 `KeyFromRune`，拒绝自造巨型枚举 schema（重复库职责）。

## 10. 变更说明

- 本设计对 `docs/mcp-tools.md` 的扩展：新增 §Initial Tool Set 之外的 5 项（4 个新工具 + find_window 增强 + 按键契约 + capture 前修复），未推翻其分层/非目标/契约。
- 阶段结束点：实现以具体 commit 为准，并在本文件 §10 追加章节号对应的实现记录。
