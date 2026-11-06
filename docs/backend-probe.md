# 后端探查与分层裁决设计（Backend Probe Design）

- 基础设计：`docs/mcp-tools.md`（适配层目标、非目标、分层、工具契约、兼容策略）
- 遵循：与 `docs/mcp-tools-gaps.md` 相同的工程原则条款（范围三分、分层铁律、失败政策、资源上限、验收分层、反过度设计）。该缺口设计引用了 `docs/ENGINEERING_PRINCIPLES.md`，但此文件未出现在本仓库当前提交中；冲突时以本文件与 `docs/mcp-tools.md` 为准
- 关联文档：`API_CN.md`、`API.md`、`docs/mcp-tools.md`、`docs/mcp-tools-gaps.md`（历史缺口设计，文件已从树中删除，提交 `2e4f3e5` 可查阅）

## 1. 背景与一句话目标

`winput` 以窗口为中心，用同一套 API 在 `BackendMessage`（`PostMessage`，可不抢焦点、不移动光标）与 `BackendHID`（Interception 内核模拟，通常移动光标且常需前台）之间切换。今日选择后端的唯一公开入口是全局 `SetBackend`。MCP 侧对应 `switch_backend`（敏感操作）。

`PostMessage` 返回成功不等于目标会消费该输入。游戏、Electron 自绘界面等常忽略窗口消息，调用仍可能成功。UIPI 会拦截低完整性进程向高完整性窗口投递的消息；UIAccess 与目标窗口的消息过滤（如 `ChangeWindowMessageFilterEx`）可构成例外，因此完整性高低不能写成「所有消息均已被阻断」的证明。系统没有可靠接口询问某 HWND 是否在听 `WM_KEYDOWN`。因此「自动判定目标是否必须走 HID」不能做成神谕，只能做成带理由的诊断。

原则：Backend Probe 根据可观察的系统与窗口特征给出启发式后端建议。它不保证目标应用会消费或响应已投递的消息。不得将诊断建议包装为输入兼容性的确定性证明。

一句话目标：**在核心库用本地系统调用给出可序列化的后端建议（建议值、特征、置信度），MCP 只把这份诊断交给调用方；核心库不引入网络或模型，MCP 本阶段也不在工具内部调用模型。**

生命周期：`ProbeBackend` / `probe_backend` 是任务初始化时的诊断，不是 `Click`、`Type`、`Press` 等输入操作的前置步骤。同一 RPA 执行序列在初始化时探测一次；调用方据此选定后端，后续操作沿用已选后端，不再重跑探针。目标应用退出并重启、目标窗口销毁并重建、任务切换到另一独立应用、交互会话或权限环境发生实质变化、或实际输入目标已不是当初探测的窗口时，须重新判断。同一应用内不同子控件，若任务级后端策略仍然适合，可继续复用；若改为机制明显不同的输入目标，则应对新目标重新探测。MCP 本阶段不引入服务端任务上下文。

## 2. 现状能力矩阵（审计结果）

| 能力 | 现状 | 相对本设计 |
|---|---|---|
| `SetBackend` / `BackendMessage` / `BackendHID` | 全局后端，HID 初始化失败有明确错误 | 已有；探查不得默默改写它 |
| `switch_backend` | MCP 敏感工具，改全局后端 | 已有；不作为按窗口裁决的落点 |
| `ErrPermissionDenied` | 注释指向 UIPI | 已有；探查应在投递前尽量预判消息路径是否可能被完整性隔离拦住 |
| `FindChildByClass` / 子窗口枚举 | `FindChildByClass` 经 `FindWindowExW` 只查找直接子窗口 | 标准控件分类须针对**探测目标 HWND 自身**的类名；后代类名另采，不得把直接或递归后代上的 `Edit`/`Button` 当成对父窗口发消息的高置信证据 |
| `Window.Text` / `Value`（含 UIA 回退） | 已有 | 可回读控件的观察手段；本阶段不把「先写入再回读」当作默认探针 |
| `SendMessageTimeoutW` | 用于 `WM_GETTEXT` 等读路径 | **不可**当作「输入已被消费」的预言机（`DefWindowProc` 也会成功返回） |
| `Window.Scroll` 在 HID 上 | 返回 `ErrDirectedScrollUnsupported` | 定向滚动不能因探查改走 HID；MCP 不得把它映射成「后端不可用」 |
| `Window.PressHotkey` 在 Message 上 | 含 Ctrl/Alt/Shift 时走 `pressHotkeyForeground`（尝试切前台并以扫描码发送） | Message 不等于全部操作都能后台完成；设计须写明 |
| 按次 / 任务级后端，且不改全局状态 | 无。公开输入方法自行获取不可重入的 `inputMutex`；`SetBackend` 只获取 `backendMutex` | **缺口**。原拟 `UseBackend(fn)` 会在持有 `inputMutex` 后再调用同样加锁的公开方法，必然自死锁，且无法隔离全局 `currentBackend`。改为任务级不可变 `Input` |
| 诊断结构（建议 + 特征 + 置信度） | 无 | **缺口**。`SuggestedBackend` 不得复用 `type Backend int`（其零值为 `BackendMessage`） |
| MCP 观察类探查工具 | 无 | **缺口** |
| MCP Sampling（`sampling/createMessage`） | 未实现；`docs/mcp-tools.md` 明确不把 planning / sampling 纳入当前适配层 | 本阶段不做 |

最大断层：调用方（含外部智能体）无法区分「消息路径大致可用」「消息路径很可能被 UIPI 拦住」「仅有弱启发」与「证据不足」，只能盲目 `switch_backend`。全局 `SetBackend` 也无法让同一进程内的不同任务各用各的后端。

## 3. 范围矩阵

| | 内容 |
|---|---|
| **现在做** | ① 核心库 `ProbeBackend`：静态采集完整性、目标窗口类名、有限后代类名、调用方 UIAccess（能读则读）、驱动是否可用来源；输出建议后端、评估状态、特征结构、置信度与原因（§4.1–4.3）；② 纯函数裁决器，供表驱动测试（§4.4）；③ 核心库任务级 `Input`：`NewInput(backend)` 绑定不可变后端，方法名与现有 `Window` 及包级输入 API 对齐（§4.5）；实现分 PR，见 §10，**不得**把 `Input` 塞进探针首个 PR；④ MCP 只读工具 `probe_backend`（§4.6）；⑤ MCP 既有变更类输入工具增加可选字段 `backend`（additive，§4.7） |
| **以后做** | 默认关闭的加深 UIA 探查（根节点以外的有限深度控件树摘要）；DirectInput / `dinput8.dll` / `dinput.dll` 模块枚举（弱提示，且跨位数时 `EnumProcessModules` 可能 `ERROR_PARTIAL_COPY`）；MCP 进程内按「可执行路径 + 类名」缓存上次高置信度结论（带失效）；调用方显式打开时才使用的 Sampling 闭环；窗口级持久偏好；MCP 服务端任务级执行上下文 |
| **永不做** | 核心库发起网络请求或依赖任何模型；把 HWND 写入 MCP 契约；反作弊模块名单作为切换依据；把 `dinput8.dll` 或权限差单独标成「高置信度必须 HID」；用完整性高低断言「全部输入消息确定被阻断」；用 `SendMessageTimeout` 成功或任意像素差当作输入已被消费；在 `click` 等变更工具内部默默问模型；默认的无害点击/按键探针（有副作用）；为本设计承诺未经测量的覆盖率或延迟数字；以 `UseBackend(fn)` 临时改写全局后端；为探查调用 `ChangeWindowMessageFilterEx` 等去修改目标窗口的消息过滤器；MCP 默认返回完整窗口标题或可执行文件绝对路径 |

本阶段必须：

- 完整性关系为 `target_higher` 时优先评估 UIPI：`MessageAssessment=likely_blocked`，原因 `uipi_likely_blocked`；仅当 HID 就绪时建议 HID，否则建议未定。此规则优先于标准控件规则
- 仅当**目标 HWND 自身**命中标准控件白名单时，才给出高置信消息建议（原因 `target_standard_control`）
- 其余情况给出低或中置信度，特征原样保留，不假装已经裁断
- `probe_backend` 为 `read_only`；可选 `backend` 不出现时，既有全局后端行为不变
- `NewInput` 只接受 `BackendMessage` 与 `BackendHID`，拒绝其它值，不得把未知整型默认成 Message

本阶段应当：文档写明 HID 建议改变语义（光标移动、通常需前台）；文档写明 Message 下带修饰键的 `PressHotkey` 可能切前台

本阶段不做：Sampling、整树 UIA、像素探针、磁盘特征库、DirectInput 模块扫描、覆盖率承诺、MCP 服务端任务上下文、名为 `auto` 的后端值

实施顺序见 §10：PR1 只加探测，不改现有输入行为；PR2 再加 `Input`；PR3 再接 MCP。

## 4. 设计

### 4.1 核心库公开接口

新增诊断类型与函数（名称以实现时导出名为准，语义不得削弱）：

- `ProbeBackend(w *Window) (*BackendProbe, error)`：`w` 为 nil 或无效则返回既有窗口类错误（如 `ErrWindowGone`），不做建议。**不要**复用 `checkReady()`：可见性与是否最小化是输入前置条件，不是静态探查条件。隐藏或最小化窗口仍允许采集静态特征。
- 本阶段默认只走静态路径，不发送鼠标键盘消息，不截屏，不遍历整棵 UIA 树。
- 探查开始时记下目标 HWND 对应的 PID、类名与进程信息，后续采集尽量沿同一进程身份。窗口在探查中消失则返回明确的窗口失效错误；非关键特征单项失败可保留 unknown。禁止把不同进程的零散信息拼成一份看似完整的诊断。

`Backend` 继续表示**实际执行**后端（`BackendMessage`、`BackendHID`）。诊断建议使用独立类型，避免与 `BackendMessage == 0` 冲突：

```text
BackendSuggestion: unspecified | message | hid
MessageAssessment: unknown | likely_usable | likely_blocked
HIDStatus: ready | driver_unavailable | dll_unavailable | unknown
Confidence: high | medium | low
```

`Confidence` 表示规则依据的强弱（high：有较强结构性依据；medium：有意义的启发；low：依据不足），不是统计概率，也不意味着实际输入成功率达到某一百分比。

`BackendProbe` 至少包含：

| 字段 | 含义 |
|---|---|
| `SuggestedBackend` | `BackendSuggestion`，未定时为 `unspecified`，禁止用「猜一个」填充 |
| `MessageAssessment` | 对消息路径的评估，三态见上；未知必须保留 |
| `HIDStatus` | 当前进程使用 Interception 的能力检查结果，区分驱动缺失、DLL 加载失败与无法确认 |
| `Confidence` | `high` / `medium` / `low` |
| `Reasons` | 稳定枚举列表，可多项 |
| `Features` | 见 §4.2 |

原因枚举（本阶段）：

| 原因 | 含义 |
|---|---|
| `uipi_likely_blocked` | 目标完整性高于当前进程，因而消息路径很可能受限；不是「已用实际投递证明全部消息被阻断」 |
| `target_standard_control` | **目标 HWND 自身**的类名命中标准控件白名单（至少包括 `Edit`、`Button`；可加白名单，变更须回写本表） |
| `canvas_class` | **目标 HWND 自身**的窗口类名属于已知整块画布（本阶段：`UnityWndClass`、`UnrealWindow`） |
| `integrity_unknown` | 无法读取完整性 |
| `insufficient_evidence` | 静态信号不足以高置信裁断 |

不采用的原因（本阶段）：`standard_child`（后代标准控件不得作为对父窗口的高置信消息证据）、`dinput_loaded` / `modules_unknown`（模块枚举延后）、反作弊模块名、`GetCursorPos` / `GetAsyncKeyState` 是否被目标调用（外部不可探）、`uipi_blocked`（改用 `uipi_likely_blocked`，避免写成既成事实）。

### 4.2 特征结构

`Features` 只保留对裁决或上层阅读有用、且可序列化的字段。首版：

- 进程名（能取到才填，取不到记空并在原因中体现，不报假值）
- `TargetClass`：探测对象自身的窗口类名
- `DescendantClasses`：后代窗口类名（去重，条数上限见 §4.8）。若实现采用 `EnumChildWindows`，须记录这是递归后代，不只是直接子窗口
- 完整性比较结果：`target_higher` / `equal` / `lower` / `unknown`
- 调用方 UIAccess：能读则填，读不到保留未知（如 `*bool` 为空）
- `ChildrenTruncated`、`ClassesTruncated`：两类截断独立标记

本阶段不采集：可执行文件绝对路径、完整窗口标题（核心库若将来对 Go 调用方提供标题，须为可选；MCP 默认不返回，见 §4.6）、完整模块表、DirectInput 短名。

禁止写入：HWND、token 句柄、完整模块路径枚举、截图像素、原始安全描述符。

不得为探查而调用 `ChangeWindowMessageFilterEx` 或进程级过滤器去修改目标行为。

MCP 输出是这份结构的子集或等同映射，字段 additive，不暴露 provider 原生句柄；默认子集见 §4.6。

### 4.3 裁决优先级（必须写明，禁止未写出的维用「常见情况」填充）

纯函数 `decideBackend(features, hidStatus) *BackendProbe`（不含系统调用）按下列顺序，先命中的规则优先生效。消息安全限制优先于控件类型：完整性隔离已成立则不得因目标是 `Edit` 就改回高置信消息。HID 不就绪时，不得把 HID 写成必然方案，应改为 `unspecified`。

1. 完整性比较为 `target_higher` → `MessageAssessment=likely_blocked`，`Confidence=high`（指结构性依据充分，不是已验证投递失败，也不是成功率承诺），原因 `uipi_likely_blocked`。须在文档与 MCP 描述中写明例外：调用方 UIAccess、目标窗口可能通过消息过滤放行特定消息。若 `HIDStatus=ready` 则 `SuggestedBackend=hid`，否则 `unspecified`（消息路径很可能不可用且 HID 不可用，由调用方决定是否提权或安装驱动）。
2. 否则，**目标 HWND 自身**类名命中标准控件白名单 → `SuggestedBackend=message`，`MessageAssessment=likely_usable`，`Confidence=high`，原因 `target_standard_control`。后代中出现 `Edit`/`Button` **不得**触发本条。调用方若实际要操作某个子控件，应先 `FindChildByClass`（或等价发现）并把该子窗口作为探测对象。high 仍只表示结构依据，不保证 `PostMessage` 会被消费。
3. 否则，**目标 HWND 自身**类名命中画布白名单 → 若 HID 就绪则 `SuggestedBackend=hid`，否则 `unspecified`；`MessageAssessment=unknown`，`Confidence=medium`，原因 `canvas_class`。不得标 high。白名单本阶段仅 `UnityWndClass`、`UnrealWindow`，不扩展成框架名数据库。
4. 否则 → `SuggestedBackend=unspecified`，`MessageAssessment=unknown`，`Confidence=low`，原因 `insufficient_evidence`，特征仍全部返回。

说明：第 3 步是启发，不是证明目标忽略消息。文档与 MCP `description` 须使用「建议」，不得使用「必须」或覆盖率数字。DirectInput 模块命中不进入本阶段规则。

### 4.4 测试缝

`decideBackend` 必须可在无窗口环境下表驱动测试。系统调用采集（完整性、枚举子窗、UIAccess、HID 初始化探测）单独封装，失败时写入 `unknown` / 对应 `HIDStatus`，不得把采集失败翻译成 HID 建议。

### 4.5 任务级 `Input`（替代原 `UseBackend`）

不采用 `UseBackend(b Backend, fn func() error) error`。现行公开输入方法均自行 `inputMutex.Lock()`，`inputMutex` 为不可重入的 `sync.Mutex`；在持锁回调中再调 `Window.Click` 等方法必然死锁。`SetBackend` 只锁 `backendMutex`，即使持有输入锁也不能隔离全局 `currentBackend`，恢复时还可能覆盖其他 goroutine 的新设置。

改为：

```text
NewInput(backend Backend) (*Input, error)
Input.backend 创建后不可变
```

`NewInput` 只接受 `BackendMessage` 与 `BackendHID`。其它整型值必须报错，不得解释成 Message。现行 `SetBackend` 对非 `BackendHID` 的整型会直接写入 `currentBackend`，随后各实现按「是否等于 `BackendHID`」分支，非法值会静默走 Message。本设计不把收紧 `SetBackend` 的对外行为视为已批准的变更；若要用同一校验函数处理 `SetBackend`，须另作说明。

`NewInput(BackendHID)` 必须在构造时走与探查相同的内部 HID 初始化入口，成功后才返回执行器；失败返回既有 `ErrDriverNotInstalled` / `ErrDLLLoadFailed`（或将来更贴切的既有错误），不留下半创建对象。

HID 初始化责任必须统一：在 `hid.Init()` 之上抽取一处内部包装（并发安全、可重复调用；`hid.Init()` 成功后再次调用本为幂等快路径），供 `ProbeBackend`（写入 `HIDStatus`，失败不使整个探针失败）、`NewInput(BackendHID)`（失败则构造失败）以及旧的 `SetBackend` / `checkBackend` 共用错误映射，禁止再写一套互不知情的初始化逻辑，也禁止在 `winput` 里另维护一份与 `hid` 内部状态脱节的「已初始化」标记。`Input` 在构造成功后，各方法不再反复调用该初始化检查；运行期设备或投递失败仍返回既有具体错误，不假设初始化一次后硬件永不出错。旧的 `Window` 路径改用同一入口后，已成功初始化时亦只走幂等快路径，不必每次重新执行完整初始化。

`Input` 方法名与现有 API 对齐，不另造 `TypeText` / `PressKey`：

窗口坐标（接收 `*Window`，语义与对应 `Window` 方法相同）：

- `Move`、`MoveRel`、`Click`、`ClickRight`、`ClickMiddle`、`DoubleClick`、`Scroll`、`KeyDown`、`KeyUp`、`Press`、`PressHotkey`、`Type`

包级全局屏幕坐标或无窗口输入，须有对应的显式后端入口，并与窗口坐标方法区分开，避免把客户区坐标与屏幕坐标混在同一方法里。至少覆盖现行：`MoveMouseTo`、`ClickMouseAt`、`DoubleClickMouseAt`、`ClickRightMouseAt`、`ClickMiddleMouseAt`、`MouseDown`、`MouseUp`、`ClickMouse`、`MoveMouseBy`、`MoveMouseToExact`、`ScrollWheel`、`PressFor`，以及无窗口的 `KeyDown` / `KeyUp` / `Press` / `PressHotkey` / `Type`。命名与现行包级函数对齐。

每次调用仍获取现有 `inputMutex`，用 `Input` 保存的后端执行，不再读取 `currentBackend`。不在整个 RPA 任务期间持有 `inputMutex`。需要独占桌面时由上层调度负责。`inputMutex` 只保证单次操作串行，不保证两个多步骤任务不会交错。

新旧 API 共存：`Window.*` 与包级函数继续使用全局默认后端。创建 `Input` 之后调用 `SetBackend` 不得改变该 `Input` 后续行为。两个 `Input` 可各用不同后端而不互相污染。

底层不必重写输入语义。现行已有接受显式 `Backend` 的无锁实现：`moveImpl`、`keyDownImpl`、`keyUpImpl`、`mouseButtonImpl`。`Click` / `ClickRight` / `ClickMiddle` / `DoubleClick` / `Scroll` / `Type` / `PressHotkey` 仍在公开方法内按 `getBackend()` 分支，**PR2 必须把它们提取为同样接受显式后端的内部实现**，不得假定底层已经全部支持。`Window.Scroll` 在 HID 上仍返回 `ErrDirectedScrollUnsupported`，不改语义，不改道为 `ScrollWheel`。Message 下带修饰键的 `PressHotkey` 仍走既有前台处理。

实现分 PR：探针首个 PR **不**实现 `Input`，见 §10。

### 4.6 MCP 工具 `probe_backend`

- **映射**：`target_id` → 注册表中的窗口 → `ProbeBackend`。
- **入参**：`target_id`（必填）。本阶段无加深探查开关。
- **出参**：建议后端（字符串枚举，未定用 `"unspecified"`）、`message_assessment`、`hid_status`、`confidence`、`reasons`、以及脱敏后的 `features`。
- **元数据**：`read_only`；无 confirmation；不要求 `-allow-mutations`。`read_only` 的含义是：不改变目标应用状态、不执行输入。允许为检查能力而调用本地 `hid.Init()`，从而在本进程加载 DLL 或初始化驱动资源；须在工具描述中写明。
- **错误**：无效 `target_id` → `target_not_found`；窗口已失效 → 既有窗口错误映射；其余 `mapLibraryError`。
- **边界**：不返回 HWND、token。默认不返回窗口标题、可执行文件绝对路径。默认可返回：进程名、`TargetClass`、`DescendantClasses`、完整性关系、调用方 UIAccess（若有）、截断标记。本阶段 MCP 提示词组装与 Sampling 都不做。
- **用法**：在 RPA 任务初始化阶段调用一次。不在 `click`、`type_text` 等输入工具内部自动调用。

### 4.7 MCP 输入工具的可选 `backend`

在既有变更类、且实际会走输入后端的工具上 **additive** 增加可选字符串字段 `backend`：`message` | `hid`（不含 `auto`）。本阶段至少覆盖与窗口绑定的 `click` / `double_click` / `right_click` / `move_mouse` / `type_text` / `press_key` / `press_hotkey`。屏幕坐标全局工具若内部也读当前后端，同样 additive 可选字段，以保持对称。

- 缺省：行为与今日相同（使用全局后端）。
- 出现：使用与 `Input` 相同的显式后端路径执行原映射；每工具携带已选定的后端，不等于每次调用重新探测。
- 非法值 → `validation_error`。
- HID 初始化失败 → `backend_unavailable`。
- `scroll`（定向滚动）若传入 `hid`：仍映射到 `Window.Scroll`（或等价的 `Input.Scroll`）且后端为 HID，库返回 `ErrDirectedScrollUnsupported` → MCP 错误码 `operation_not_supported`（新增；含义是该后端不支持此操作）。**不要**继续映射为既有的 `backend_unavailable`（那是当前 `mapLibraryError` 的行为，本设计将其视为应修正的语义错误）。不改道为全局滚轮。定向滚动与系统级滚轮不是同一件事。

不在本阶段新增名为 `auto` 的取值，避免工具内部隐藏裁断。不在本阶段增加 MCP 服务端任务上下文（上下文 ID、过期、多任务隔离、断线清理），以免为少传一个字符串引入状态机。

外部智能体的预期用法：先 `probe_backend`，再在后续输入调用中显式传入同一 `backend`。`confidence` 为 `high` 时，调用方可优先采用建议，但仍不保证实际输入成功；`low`/`medium` 时结合自身对软件的先验再选择。这是本设计采纳的「路径一」。

### 4.8 资源上限

不按系统返回的子窗数量或模块数量预分配无界切片。

| 限额 | 初值 | 含义 |
|---|---|---|
| 子控件类名收集 | 32 个去重类名 | 超出截断，置 `ClassesTruncated` |
| 子窗枚举 | 256 个 | 超出停止继续枚举，置 `ChildrenTruncated` |
| UIA 深度 | 本阶段 0（不走加深） | 以后做时须另定深度与节点上限 |

二者不是同一件事，必须分开标记。无延迟数字承诺。静态路径不得发起网络。本阶段不做模块全表枚举。

## 5. 分层与铁律

- 核心库只做本地诊断与输入。禁止 `net`、模型 SDK、对 MCP 类型的引用。外壳（`cmd/mcp-server`、`internal/mcpadapter`）不得把 JSON-RPC 或 `target_id` 泄漏进核心类型。
- MCP 适配层只作为 `ProbeBackend` 与（PR2 之后）`Input` 的消费者。允许为此增加核心库公共 API，因为 Go 调用方同样需要诊断与任务级后端；这不是「仅为 MCP 形状而改库」。禁止只为 Sampling 或提示词增加核心 API。
- 特征结构是诊断领域模型，可 JSON 序列化，供快照或表驱动测试。采集失败保留为 unknown，不在采集层丢弃。
- HWND 等 provider 原生句柄不得进入 MCP 契约（承袭 `docs/mcp-tools.md`）。MCP 默认不返回标题与绝对路径。
- 工具采用 additive 演进：可选 `backend` 不得变为必填，不得删除既有字段（承袭兼容策略）。
- 铁律：探查不得有输入副作用（不点击、不打字、不截屏作为本阶段实现）。本地 HID 能力检查可以初始化本进程的驱动/DLL，但不得向目标窗口发送输入。要改须重新评审并回写本节。
- 铁律：不得把全局 `SetBackend` 当作按窗口或按任务裁决的实现。
- 铁律：不得以实现 `UseBackend(fn)` 的方式临时改写全局后端。
- 铁律：输入工具不得在内部自动调用 `ProbeBackend`。

与 `docs/mcp-tools-gaps.md`「永不做：为核心库增加 MCP 专用接口」的关系：本设计增加的是输入诊断与任务级后端执行器，供所有调用方使用，不视为 MCP 专用接口。若实现时出现仅被 adapter 引用、Go 文档无法解释的符号，则视为违例，须删或改设计。

## 6. 失败政策

- `ProbeBackend`：窗口无效 → 返回错误，无部分建议。完整性或 UIAccess 采集失败 → 成功返回诊断，对应字段为 unknown，并附原因；这是对象内降级，不是任务失败。HID 初始化失败 → 写入对应 `HIDStatus`，探针本身仍成功。
- 截断（子窗/类名达到上限）→ 成功返回，分别标记 `ChildrenTruncated` / `ClassesTruncated`；不得把截断结果标成 `high`（若某条高置信规则依赖「未看到某类特征」，截断后最高只许 `medium` 或 `low`）。`target_standard_control` 仅在目标自身类名命中时成立，不得从「截断后的后代列表里没看到 Edit」推断父窗口适合或不适合 Message。
- `NewInput`：非法 `Backend` 或 HID 初始化失败则不返回可用执行器，返回明确错误。
- MCP：单个工具失败走既有 `code` / `retryable` / `details`，不中断 server。`ErrDirectedScrollUnsupported` 映射为 `operation_not_supported`，与驱动/DLL 不可用的 `backend_unavailable` 分开。
- 空进程名、未知完整性是合法诊断成功，不是错误。

硬告警：操作未执行（后端不可用、窗口无效、操作不被该后端支持）。软告警：采集不全、列表截断。二者不得混用同一原因码的软硬含义。

## 7. 已知限制

- 静态分类不能证明目标会忽略或接受消息；`medium`/`low` 需要调用方判断。`high` 也只表示依据充分，不保证成功。
- 未测量「高置信规则覆盖真实窗口的比例」，不对外承诺。
- 本阶段不做 UIA 加深，自绘控件若自身不是标准类名、也没有画布类名，将落入 `low`。
- 画布类名仅为启发。
- `Input` 使用 HID 仍具 HID 既有语义：移动光标、调用可能阻塞、通常需前台。
- 定向 `scroll` 不能用本设计「自动改 HID」来完成。
- Message 后端下，带修饰键的 `PressHotkey` 可能需要把目标窗口提到前台；不能认为选择 Message 就等于全程后台。
- 完整性读取在跨完整性时可能失败，设计已按 unknown 处理；失败频率未经本仓库实测。
- 一次探测结果的有效范围限于同一应用、同一交互环境、同一执行序列；见 §1。
- MCP 本阶段不缓存结论、不问模型、不保存任务级后端状态。

## 8. 验证与验收

| 项目 | 方法 | 通过标准 |
|---|---|---|
| 裁决优先级（§4.3） | 表驱动单元测试：构造特征 → 建议/评估/置信度/原因 | 含 UIPI 优先于目标自身为 Edit；后代 Edit/Button 不得使父窗口高置信 Message；目标自身为 Edit/Button 才触发 `target_standard_control`；未知完整性不得当成 UIPI；HID 不就绪时画布规则不得强行建议 HID；截断不得冒充高置信 |
| 特征序列化 | 结构断言或快照（无 HWND、无运行时句柄） | 快照不含 HWND、token、绝对不稳定地址；MCP 映射不含标题与绝对路径 |
| `NewInput` 隔离（PR2） | 单元测试 | `NewInput` 拒绝非法 Backend；HID 初始化失败则构造失败；创建后 `SetBackend` 不影响该对象；两个 `Input` 后端互不污染；并发调用时单次操作仍受 `inputMutex` 保护；HID 定向滚动仍为 `ErrDirectedScrollUnsupported`；Message 修饰键快捷键保留前台语义。**删除**原「`UseBackend` 恢复后 `SetBackend` 观察值不变」的验收项 |
| `probe_backend` schema | `tools/list` 结构断言 | 含出参字段与 `read_only` |
| 可选 `backend` | schema 为可选；缺省行为单测或映射测试；非法值 `validation_error` | 通过 |
| Scroll 错误映射 | 映射测试 | `ErrDirectedScrollUnsupported` → `operation_not_supported`，不再是 `backend_unavailable` |
| 门控 | 无 `-allow-mutations` 时变更工具仍 `unsafe_operation`；`probe_backend` 在默认只读模式下可调用 | 通过 |
| 真实窗口 | 真实桌面人工冒烟（记事本 Edit、标准 Button、Electron、自绘画布、高完整性目标；记录探测结果、选用后端、操作、是否成功、前台状态与实际错误。本机无合适样本则记入限制） | 人工记录，不进 CI；不以单元测试阶段的完成去承诺某成功率 |

视觉一致性、覆盖率、延迟数字不进入本阶段完成标准。`inputMutex` 不保证两个 RPA 多步骤序列互不交错，不列入库内验收为「任务独占」。

## 9. 反过度设计复核

- 拒绝在核心库或默认 MCP 路径引入模型。分层已经把「语义精裁」交给调用方已有的客户端模型（路径一），不必在服务端再嵌一套 Sampling。
- 拒绝默认微探针（点击、像素差、`SendMessageTimeout` 当预言机）：副作用大，且假阳性/假阴性已在评审中说明。
- 拒绝反作弊名单与「十之七八」类无测量承诺。
- 拒绝为每个输入方法复制一套 `ClickHID`/`ClickMessage` API；也拒绝用临时改全局变量的 `UseBackend(fn)`。用任务级 `Input` 一处绑定后端，方法名与现有 API 对齐。
- 本阶段不做 UIA 整树：现有 `Value()` 已覆盖「能读就读」；探树是以后做的可选加深，避免把 COM 初始化与超时带进每次诊断。
- 本阶段不做 DirectInput 模块扫描：加载模块不等于当前用它收输入，且跨位数枚举易失败。
- 不新增 `auto` 后端值，避免未实现的自动裁断混进契约。
- 不把首版做成庞大的窗口类名或框架数据库。
- 不为少传一个 `backend` 字符串而在 MCP 服务端引入任务状态机。

## 10. 变更说明

- 本设计扩展 `docs/mcp-tools.md`：在不推翻其非目标（无 agent runtime、无规划语义、无 HWND 契约）的前提下，增加只读诊断工具与既有输入工具的可选 `backend` 字段，并修正定向滚动的错误映射。
- 相对缺口设计「不为核心库增加 MCP 专用接口」：见 §5 的澄清。
- 相对初稿（提交 `d4ce95a`）的修正：删除 `UseBackend(fn)`，改为任务级 `Input`；`SuggestedBackend` 独立于 `Backend`；`standard_child` 改为仅针对目标 HWND 的 `target_standard_control`；UIPI 保留最高优先级但改为 `likely_blocked` / `uipi_likely_blocked`；补齐任务生命周期；统一 HID 初始化入口；MCP 默认脱敏；Scroll 映射为 `operation_not_supported`；DirectInput 延后；方法名与现有 API 对齐。
- 相对评审中讨论而未采纳的项：Sampling 默认闭环、UIA 每次探查、像素探针、dinput/UIPI 直接写成「必须 HID」或「全部消息确定阻断」、每击自动探测、MCP 服务端任务上下文，均列入永不做或以后做，见 §3。
- 实施顺序（不得合并成一个 PR）：
  1. **PR1**：仅 `ProbeBackend`、诊断类型、`decideBackend`、表驱动测试与资源截断。不修改全局后端，不产生实际输入，不实现 `Input`。
  2. **PR2**：`NewInput` 与对齐后的方法、提取 Click/Type/Scroll/PressHotkey 等内部实现、HID 初始化前移并与探针共用入口、隔离测试。保留 `Window` 方法与 `SetBackend`。
  3. **PR3**：`probe_backend`、输入工具可选 `backend`、错误码 `operation_not_supported`、MCP 特征脱敏、schema 与门控测试。不加 `auto`，不引入服务端任务状态。
- 阶段结束点：实现以具体 commit 为准，并在本节追加对应实现记录。本文件在用户确认前不得视为已批准开工的编码任务。

## 11. 尚未证实（不得混入已确定设计）

| 事项 | 影响 | 当前应对 |
|---|---|---|
| 完整性读取与 UIAccess 读取在常见目标上的成功率 | 许多窗口可能长期停在 unknown | unknown 保留；不把失败当 HID |
| 画布类名白名单是否过窄或过宽 | medium 建议可能不准 | 白名单变更须回写 §4.3 并补测试 |
| 标准控件目标上 Message 的实际消费率 | high 仍可能失败 | high 只表示结构依据；冒烟记录，不承诺比例 |
| 调用方模型根据特征摘要做精裁的实际收益 | 不影响本阶段库接口 | 由 MCP 用户使用后评估，不作为本阶段验收 |
