# 后端探查与分层裁决设计（Backend Probe Design）

- 基础设计：`docs/mcp-tools.md`（适配层目标、非目标、分层、工具契约、兼容策略）
- 遵循：与 `docs/mcp-tools-gaps.md` 相同的工程原则条款（范围三分、分层铁律、失败政策、资源上限、验收分层、反过度设计）。该缺口设计引用了 `docs/ENGINEERING_PRINCIPLES.md`，但此文件未出现在本仓库当前提交中；冲突时以本文件与 `docs/mcp-tools.md` 为准
- 关联文档：`API_CN.md`、`API.md`、`docs/mcp-tools.md`、`docs/mcp-tools-gaps.md`（历史缺口设计，文件已从树中删除，提交 `2e4f3e5` 可查阅）

## 1. 背景与一句话目标

`winput` 以窗口为中心，用同一套 API 在 `BackendMessage`（`PostMessage`，可不抢焦点、不移动光标）与 `BackendHID`（Interception 内核模拟，通常移动光标且常需前台）之间切换。今日选择后端的唯一公开入口是全局 `SetBackend`。MCP 侧对应 `switch_backend`（敏感操作）。

`PostMessage` 返回成功不等于目标会消费该输入。游戏、Electron 自绘界面等常忽略窗口消息，调用仍可能成功。UIPI 会拦截低完整性进程向高完整性窗口投递的消息。系统没有可靠接口询问某 HWND 是否在听 `WM_KEYDOWN`。因此「自动判定目标是否必须走 HID」不能做成神谕，只能做成带理由的诊断。

一句话目标：**在核心库用本地系统调用给出可序列化的后端建议（建议值、特征、置信度），MCP 只把这份诊断交给调用方；核心库不引入网络或模型，MCP 本阶段也不在工具内部调用模型。**

## 2. 现状能力矩阵（审计结果）

| 能力 | 现状 | 相对本设计 |
|---|---|---|
| `SetBackend` / `BackendMessage` / `BackendHID` | 全局后端，HID 初始化失败有明确错误 | 已有；探查不得默默改写它 |
| `switch_backend` | MCP 敏感工具，改全局后端 | 已有；不作为按窗口裁决的落点 |
| `ErrPermissionDenied` | 注释指向 UIPI | 已有；探查应在投递前尽量预判消息是否会被完整性隔离拦住 |
| `FindChildByClass` / 子窗口枚举 | 已有 | 标准控件分类的依据 |
| `Window.Text` / `Value`（含 UIA 回退） | 已有 | 可回读控件的观察手段；本阶段不把「先写入再回读」当作默认探针 |
| `SendMessageTimeoutW` | 用于 `WM_GETTEXT` 等读路径 | **不可**当作「输入已被消费」的预言机（`DefWindowProc` 也会成功返回） |
| `Window.Scroll` 在 HID 上 | 返回 `ErrDirectedScrollUnsupported` | 定向滚动不能因探查改走 HID |
| 按次后端覆盖 / 窗口级偏好 | 无 | **缺口** |
| 诊断结构（建议 + 特征 + 置信度） | 无 | **缺口** |
| MCP 观察类探查工具 | 无 | **缺口** |
| MCP Sampling（`sampling/createMessage`） | 未实现；`docs/mcp-tools.md` 明确不把 planning / sampling 纳入当前适配层 | 本阶段不做 |

最大断层：调用方（含外部智能体）无法区分「消息路径可用」「消息路径被 UIPI 拦住」「仅有弱启发」与「证据不足」，只能盲目 `switch_backend`。

## 3. 范围矩阵

| | 内容 |
|---|---|
| **现在做** | ① 核心库 `ProbeBackend`：静态采集完整性、窗口类名、有限子控件类名、可选的已知模块名探测、驱动是否可用来源；输出建议后端、特征结构、置信度与原因（§4.1–4.3）；② 纯函数裁决器，供表驱动测试（§4.4）；③ 核心库 `UseBackend`：在持有既有输入锁的前提下按次覆盖后端并恢复，不改变调用返回后的全局 `SetBackend`（§4.5）；④ MCP 只读工具 `probe_backend`（§4.6）；⑤ MCP 既有变更类输入工具增加可选字段 `backend`（additive，§4.7） |
| **以后做** | 默认关闭的加深 UIA 探查（根节点以外的有限深度控件树摘要）；MCP 进程内按「可执行路径 + 类名」缓存上次高置信度结论（带失效）；调用方显式打开时才使用的 Sampling 闭环；窗口级持久偏好 |
| **永不做** | 核心库发起网络请求或依赖任何模型；把 HWND 写入 MCP 契约；反作弊模块名单作为切换依据；把 `dinput8.dll` 或权限差单独标成「高置信度必须 HID」；用 `SendMessageTimeout` 成功或任意像素差当作输入已被消费；在 `click` 等变更工具内部默默问模型；默认的无害点击/按键探针（有副作用）；为本设计承诺未经测量的覆盖率或延迟数字 |

本阶段必须：

- 两个高置信度桶可判定：标准 Win32 子控件 → 建议消息；UIPI/完整性使消息不可用 → 标「消息不可用」，仅当驱动可用时建议 HID
- 其余情况给出低或中置信度，特征原样保留，不假装已经裁断
- `probe_backend` 为 `read_only`；可选 `backend` 不出现时，既有全局后端行为不变

本阶段应当：文档写明 HID 建议改变语义（光标移动、通常需前台）

本阶段不做：Sampling、整树 UIA、像素探针、磁盘特征库、覆盖率承诺

## 4. 设计

### 4.1 核心库公开接口

新增诊断类型与函数（名称以实现时导出名为准，语义不得削弱）：

- `ProbeBackend(w *Window) (*BackendProbe, error)`：`w` 无效则返回既有窗口类错误，不做建议。
- 本阶段默认只走静态路径，不发送鼠标键盘消息，不截屏，不遍历整棵 UIA 树。

`BackendProbe` 至少包含：

| 字段 | 含义 |
|---|---|
| `SuggestedBackend` | `BackendMessage`、`BackendHID`，或表示未建议的明确零值/未定（实现用可文档化的零值，禁止用「猜一个」填充） |
| `MessageUsable` | 三态：可用 / 不可用 / 未知（未知必须保留，不得在解析时丢弃） |
| `HIDAvailable` | 当前进程是否已能初始化或探测到 Interception（沿用既有驱动/DLL 检测，不发明新通道） |
| `Confidence` | `high` / `medium` / `low` |
| `Reasons` | 稳定枚举列表，可多项 |
| `Features` | 见 §4.2 |

原因枚举（本阶段）：

| 原因 | 含义 |
|---|---|
| `uipi_blocked` | 目标完整性高于当前进程，或等价的完整性隔离使消息路径不可用 |
| `standard_child` | 发现标准子控件类名（至少包括 `Edit`、`Button`；可加白名单，变更须回写本表） |
| `canvas_class` | 窗口类名属于已知整块画布（本阶段：`UnityWndClass`、`UnrealWindow`） |
| `dinput_loaded` | 在可枚举的模块表中见到 `dinput8.dll` 或 `dinput.dll`（弱提示） |
| `modules_unknown` | 无法枚举模块（权限、位数或 `OpenProcess` 失败） |
| `integrity_unknown` | 无法读取完整性 |
| `insufficient_evidence` | 静态信号不足以高置信裁断 |

不采用的原因：反作弊模块名、`GetCursorPos` / `GetAsyncKeyState` 是否被目标调用（外部不可探）。

### 4.2 特征结构

`Features` 只保留对裁决或上层阅读有用、且可序列化的字段：

- 进程名、可执行路径（能取到才填，取不到记空并在原因中体现，不报假值）
- 窗口标题、类名
- 完整性比较结果：`target_higher` / `equal` / `lower` / `unknown`
- 子控件类名列表（去重，条数上限见 §4.8）
- 命中的已知模块短名（仅白名单，不返回完整模块表）
- `HIDAvailable` 的事实来源与 `Features` 重复时，以顶层字段为准

禁止写入：HWND、token 句柄、完整模块路径枚举、截图像素、原始安全描述符。

MCP 输出是这份结构的子集或等同映射，字段 additive，不暴露 provider 原生句柄。

### 4.3 裁决优先级（必须写明，禁止未写出的维用「常见情况」填充）

纯函数 `decideBackend(features) *BackendProbe`（不含系统调用）按下列顺序，先命中的高置信规则优先生效；并列时取更保守的「消息不可用」优先于「建议消息」（完整性隔离已成立则不得因存在 `Edit` 子窗就改回高置信消息——UIPI 仍会拦住投递）：

1. 完整性比较为 `target_higher` → `MessageUsable=false`，`Confidence=high`，原因 `uipi_blocked`。若 `HIDAvailable` 则 `SuggestedBackend=BackendHID`，否则 `SuggestedBackend` 为未定（消息不可用且 HID 不可用，由调用方决定是否提权或安装驱动）。
2. 否则，子控件类名命中标准控件白名单 → `SuggestedBackend=BackendMessage`，`MessageUsable=true`，`Confidence=high`，原因 `standard_child`。
3. 否则，窗口类名命中画布白名单 → `SuggestedBackend=BackendHID`（若 `HIDAvailable`，否则未定），`MessageUsable=unknown`，`Confidence=medium`，原因 `canvas_class`。不得标 high。
4. 否则，模块白名单命中 dinput → 同画布：建议 HID 且仅 medium，原因 `dinput_loaded`。不得标 high。单独出现 `modules_unknown` 不改变建议，只附加原因。
5. 否则 → `SuggestedBackend` 未定，`MessageUsable=unknown`，`Confidence=low`，原因 `insufficient_evidence`，特征仍全部返回。

说明：第 3、4 步是启发，不是证明目标忽略消息。文档与 MCP `description` 须使用「建议」，不得使用「必须」或覆盖率数字。

### 4.4 测试缝

`decideBackend` 必须可在无窗口环境下表驱动测试。系统调用采集（完整性、枚举子窗、枚举模块）单独封装，失败时写入 `unknown` / `modules_unknown`，不得把采集失败翻译成 HID。

### 4.5 `UseBackend`

新增类似 `UseBackend(b Backend, fn func() error) error` 的作用域接口：

- 获取既有全局输入锁（与当前输入序列化同一把锁，具体锁名以代码为准）
- 保存当前后端，切换到 `b`（HID 初始化失败则返回既有 `ErrDriverNotInstalled` / `ErrBackendUnavailable` / `ErrDLLLoadFailed`，不执行 `fn`，不留下半切换状态）
- 执行 `fn`，以 defer 恢复原后端
- 调用返回后，全局 `SetBackend` 对外部观察者不变

`Window.Scroll` 等在 HID 上本就不能定向完成的操作，在 `UseBackend(BackendHID, ...)` 内仍返回既有 `ErrDirectedScrollUnsupported`，不改语义。

不采用：为每个 `Click`/`Type` 增加必填后端参数（破坏现有签名）；探测成功后调用全局 `SetBackend`（记事本与游戏混用会互相污染）。

### 4.6 MCP 工具 `probe_backend`

- **映射**：`target_id` → 注册表中的窗口 → `ProbeBackend`。
- **入参**：`target_id`（必填）。本阶段无加深探查开关。
- **出参**：建议后端（字符串枚举，未定用明确值如 `"unspecified"`）、`message_usable`（布尔或 `null`）、`hid_available`、`confidence`、`reasons`、`features`。
- **元数据**：`read_only`；无 confirmation；不要求 `-allow-mutations`。
- **错误**：无效 `target_id` → `target_not_found`；窗口已失效 → 既有窗口错误映射；其余 `mapLibraryError`。
- **边界**：不返回 HWND；标题与路径按库采集结果原样给出（调用方自行注意敏感信息）。本阶段 MCP 提示词组装与 Sampling 都不做，故不在服务端再做二次脱敏政策；若以后做 Sampling，须另开评审。

### 4.7 MCP 输入工具的可选 `backend`

在既有变更类、且实际会走输入后端的工具上 **additive** 增加可选字符串字段 `backend`：`message` | `hid`（不含 `auto`）。本阶段至少覆盖与窗口绑定的 `click` / `double_click` / `right_click` / `move_mouse` / `type_text` / `press_key` / `press_hotkey`。屏幕坐标全局工具若内部也读当前后端，同样 additive 可选字段，以保持对称。

- 缺省：行为与今日相同（使用全局后端）。
- 出现：在 `UseBackend` 中执行原映射。
- 非法值 → `validation_error`。
- HID 不可用 → `backend_unavailable`。
- `scroll`（定向滚动）若传入 `hid`：仍映射到 `Window.Scroll` + `UseBackend(HID)`，库返回 `ErrDirectedScrollUnsupported` → 既有 `backend_unavailable`，不改道为全局滚轮。

不在本阶段新增名为 `auto` 的取值，避免工具内部隐藏裁断。

外部智能体的预期用法：先 `probe_backend`，若 `confidence` 为 `high` 可直接采信建议；若为 `low`/`medium`，结合自身对软件的先验，在后续调用里显式传入 `backend`。这是本设计采纳的「路径一」。

### 4.8 资源上限

不按系统返回的子窗数量或模块数量预分配无界切片。

| 限额 | 初值 | 含义 |
|---|---|---|
| 子控件类名收集 | 32 个去重类名 | 超出截断，特征中标记截断 |
| 子窗枚举 | 256 个 | 超出停止继续枚举，标记截断 |
| 模块白名单探测 | 只匹配固定短名单，不保存全表 | 枚举失败则 `modules_unknown` |
| UIA 深度 | 本阶段 0（不走加深） | 以后做时须另定深度与节点上限 |

无延迟数字承诺。静态路径不得发起网络。

## 5. 分层与铁律

- 核心库只做本地诊断与输入。禁止 `net`、模型 SDK、对 MCP 类型的引用。外壳（`cmd/mcp-server`、`internal/mcpadapter`）不得把 JSON-RPC 或 `target_id` 泄漏进核心类型。
- MCP 适配层只作为 `ProbeBackend` 与 `UseBackend` 的消费者。允许为此增加核心库公共 API，因为 Go 调用方同样需要诊断与按次覆盖；这不是「仅为 MCP 形状而改库」。禁止只为 Sampling 或提示词增加核心 API。
- 特征结构是诊断领域模型，可 JSON 序列化，供快照或表驱动测试。采集失败保留为 unknown，不在采集层丢弃。
- HWND 等 provider 原生句柄不得进入 MCP 契约（承袭 `docs/mcp-tools.md`）。
- 工具采用 additive 演进：可选 `backend` 不得变为必填，不得删除既有字段（承袭兼容策略）。
- 铁律：探查不得有输入副作用（不点击、不打字、不截屏作为本阶段实现）。要改须重新评审并回写本节。
- 铁律：不得把全局 `SetBackend` 当作按窗口裁决的实现。

与 `docs/mcp-tools-gaps.md`「永不做：为核心库增加 MCP 专用接口」的关系：本设计增加的是输入诊断与按次后端作用域，供所有调用方使用，不视为 MCP 专用接口。若实现时出现仅被 adapter 引用、Go 文档无法解释的符号，则视为违例，须删或改设计。

## 6. 失败政策

- `ProbeBackend`：窗口无效 → 返回错误，无部分建议。完整性或模块采集失败 → 成功返回诊断，对应字段为 unknown，并附原因；这是对象内降级，不是任务失败。
- 截断（子窗/类名达到上限）→ 成功返回，特征标记截断；不得把截断结果标成 `high`（若高置信规则依赖「未看到标准控件」，截断后最高只许 `medium` 或 `low`——本阶段因此规定：`standard_child` 仅在实际见到白名单类名时成立；未见不得从截断推断「没有标准控件」为高置信画布）。
- `UseBackend`：切换失败则不执行操作，恢复原后端（若尚未切换则保持原状），返回库错误。
- MCP：单个工具失败走既有 `code` / `retryable` / `details`，不中断 server。
- 空标题、未知完整性是合法诊断成功，不是错误。

硬告警：操作未执行（后端不可用、窗口无效）。软告警：采集不全、列表截断。二者不得混用同一原因码的软硬含义。

## 7. 已知限制

- 静态分类不能证明目标会忽略或接受消息；`medium`/`low` 需要调用方判断。
- 未测量「高置信两桶覆盖真实窗口的比例」，不对外承诺。
- 本阶段不做 UIA 加深，自绘控件若没有标准子窗、也没有画布类名，将落入 `low`。
- `dinput` 与画布类名仅为启发。
- `UseBackend(HID)` 仍具 HID 既有语义：移动光标、调用可能阻塞、通常需前台。
- 定向 `scroll` 不能用本设计「自动改 HID」来完成。
- 完整性读取与模块枚举在跨完整性、跨位数时可能失败，设计已按 unknown 处理；失败频率未经本仓库实测。
- MCP 本阶段不缓存结论、不问模型。

## 8. 验证与验收

| 项目 | 方法 | 通过标准 |
|---|---|---|
| 裁决优先级（§4.3） | 表驱动单元测试：构造特征 → 建议/置信度/原因 | 含 UIPI 优先于 Edit、dinput 不得 high、截断不得冒充「无标准控件」的高置信画布、未知完整性不得当成 UIPI |
| 特征序列化 | 结构断言或快照（无 HWND、无运行时句柄） | 快照不含 HWND、token、绝对不稳定地址 |
| `UseBackend` 恢复 | 单元测试：成功路径与切换失败路径后 `SetBackend` 观察值复原 | 通过 |
| `probe_backend` schema | `tools/list` 结构断言 | 含出参字段与 `read_only` |
| 可选 `backend` | schema 为可选；缺省行为单测或映射测试；非法值 `validation_error` | 通过 |
| 门控 | 无 `-allow-mutations` 时变更工具仍 `unsafe_operation`；`probe_backend` 在默认只读模式下可调用 | 通过 |
| 真实窗口 | 真实桌面人工冒烟（记事本应 high+message；本机无合适样本则记入限制，不阻断本阶段以单元测试为准的完成标准） | 人工记录，不进 CI |

视觉一致性、覆盖率、延迟数字不进入本阶段完成标准。

## 9. 反过度设计复核

- 拒绝在核心库或默认 MCP 路径引入模型。分层已经把「语义精裁」交给调用方已有的客户端模型（路径一），不必在服务端再嵌一套 Sampling。
- 拒绝默认微探针（点击、像素差、`SendMessageTimeout` 当预言机）：副作用大，且假阳性/假阴性已在评审中说明。
- 拒绝反作弊名单与「十之七八」类无测量承诺。
- 拒绝为每个输入方法复制一套 `ClickHID`/`ClickMessage` API，用 `UseBackend` 一处作用域解决。
- 本阶段不做 UIA 整树：现有 `Value()` 已覆盖「能读就读」；探树是以后做的可选加深，避免把 COM 初始化与超时带进每次诊断。
- 不新增 `auto` 后端值，避免未实现的自动裁断混进契约。

## 10. 变更说明

- 本设计扩展 `docs/mcp-tools.md`：在不推翻其非目标（无 agent runtime、无规划语义、无 HWND 契约）的前提下，增加只读诊断工具与既有输入工具的可选 `backend` 字段。
- 相对缺口设计「不为核心库增加 MCP 专用接口」：见 §5 的澄清。
- 相对评审中讨论而未采纳的项：Sampling 默认闭环、UIA 每次探查、像素探针、dinput/UIPI 直接高置信 HID，均列入永不做或以后做，见 §3。
- 阶段结束点：实现以具体 commit 为准，并在本节追加对应实现记录。本文件在用户确认前不得视为已批准开工的编码任务。

## 11. 尚未证实（不得混入已确定设计）

| 事项 | 影响 | 当前应对 |
|---|---|---|
| 完整性与模块枚举在常见目标上的成功率 | 许多窗口可能长期停在 unknown | unknown 保留；不把失败当 HID |
| 画布类名白名单是否过窄或过宽 | medium 建议可能不准 | 白名单变更须回写 §4.3 并补测试 |
| 调用方模型根据特征摘要做精裁的实际收益 | 不影响本阶段库接口 | 由 MCP 用户使用后评估，不作为本阶段验收 |
