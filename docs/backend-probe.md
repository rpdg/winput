# 后端环境诊断与任务级输入（Backend Probe 缩减稿）

- 基础设计：`docs/mcp-tools.md`（适配层目标、非目标、分层、工具契约、兼容策略）
- 遵循：与 `docs/mcp-tools-gaps.md` 相同的工程原则条款（范围三分、分层铁律、失败政策、资源上限、验收分层、反过度设计）。该缺口设计引用了 `docs/ENGINEERING_PRINCIPLES.md`，但此文件未出现在本仓库当前提交中；冲突时以本文件与 `docs/mcp-tools.md` 为准
- 关联文档：`API_CN.md`、`API.md`、`docs/mcp-tools.md`、`docs/mcp-tools-gaps.md`（历史缺口设计，提交 `2e4f3e5` 可查阅）
- 本稿取代提交 `330821a` 所载的完整推荐引擎方案：那一版的技术约束（UIPI 措辞、目标 HWND 与后代控件的区分、`UseBackend` 不可行、`Backend` 零值冲突等）在恢复推荐功能时仍须遵守，见 §12

## 1. 背景与一句话目标

`winput` 以窗口为中心，在 `BackendMessage` 与 `BackendHID`（Interception 系统级输入，通常需前台并移动光标）之间切换。二者不是简单的优劣，而是不同机制。

`BackendMessage` 在窗口目标 API 中主要采用窗口消息机制（一定条件下可后台、不移动真实光标）。对于屏幕坐标和无窗口 API，则依照各自实现使用对应的 Win32 输入机制，并非一律 `PostMessage`：例如现行 `MoveMouseTo` 使用 `SetCursorPos`，全局点击路径使用 `mouse_event`，无窗口 `Type` 在非 HID 时使用 `SendInput` 的 Unicode 输入。今日公开的后端入口是全局 `SetBackend`；MCP 侧对应敏感工具 `switch_backend`。

调用方实际要回答三件事：

1. 目标应用能否有效处理 Message？
2. 当前环境能否使用 HID？
3. 当前任务需要哪种输入语义（可否抢前台、要不要定向滚动、可靠性与后台约束如何权衡）？

Windows 没有可靠的静态接口，可以证明任意 HWND 会消费某种输入消息。`PostMessage` 成功也不等于目标处理了输入。因此第 3 问不能由窗口静态特征单独决定：最佳后端是应用、操作类型与任务约束的函数，不是窗口特征的函数。例如定向 `Window.Scroll` 在 HID 上返回 `ErrDirectedScrollUnsupported`；Message 下带 Ctrl/Alt/Shift 的 `PressHotkey` 会走 `pressHotkeyForeground`，可能切换前台。窗口门面与桌面门面即使方法名相近，底层语义也不相同，不能互相替代。

一句话目标：**核心库用本地系统调用报告可观察的环境事实（完整性关系、UIAccess、目标窗口类名、HID 初始化状态）；不在本阶段根据这些事实推荐该用哪种后端。任务级 `Input` 是独立的状态隔离能力，不依赖探针。**

原则：能够明确检测的，给出事实；无法明确检测的，保留未知。已通过实际验证的应用，由调用方直接配置后端。不得为了消灭「未知」而叠加启发式规则。不得将诊断包装成输入兼容性的确定性证明，也不得把探针写成每条 RPA 的必经步骤。

## 2. 现状能力矩阵（审计结果）

| 能力 | 现状 | 相对本设计 |
|---|---|---|
| `SetBackend` / `switch_backend` | 进程级全局后端；HID 初始化失败有明确错误 | 已有；诊断不得默默改写它 |
| `ErrPermissionDenied` | 注释指向 UIPI | 已有；诊断可在投递前报告完整性隔离风险 |
| `FindChildByClass` | `FindWindowExW`，只找直接子窗口 | 发现子控件后应对该子 HWND 本身采集类名，不得把后代标准控件当成父窗口的输入证明 |
| `Window.Scroll` + HID | `ErrDirectedScrollUnsupported` | 操作类型会影响后端选择；诊断不应据此「建议改 HID 以完成滚动」 |
| `Window.PressHotkey` + Message | 含修饰键时可能切前台 | Message 不等于全部操作都能后台完成 |
| 任务级、不可变的后端执行器 | 无 | **独立缺口**（§5），与探针无关 |
| 环境事实诊断（完整性、类名、HID 状态） | 无 | **本阶段缺口**（§4） |
| 由静态规则产出 `SuggestedBackend` | 无 | **本阶段不做**；待真实样本证明收益后再议（§9） |
| MCP 观察类诊断工具 | 无 | **按需**，不作为默认工具链的必经步（§4.6） |

## 3. 范围矩阵

| | 内容 |
|---|---|
| **现在做（设计）** | ① 核心库轻量诊断 `ProbeBackend`：报告完整性关系、调用方 UIAccess（能读则读，读不到为未知）、目标 HWND 自身类名、HID 初始化状态；不产出后端建议（§4）；② 任务级 `Input` 的接口与约束（§5，含 `WindowInput` / `DesktopInput` 分型），是否立即编码单独评估，见 §10 |
| **以后做** | 用 5～10 个真实应用、以业务结果（而非 API 返回成功）验证之后，再考虑恢复 `SuggestedBackend` 裁决引擎；后代窗口类名作为可选结构信息（不参与父窗口后端判断）；DirectInput 模块枚举；MCP `probe_backend`（有真实使用需求时，默认仍只返回事实）；MCP 输入工具可选 `backend` 并内部改用任务级 `Input`（与核心库 `NewInput` 分阶段，见 §5.2、§10）；有限深度 UIA、特征缓存、Sampling；将 `ErrDirectedScrollUnsupported` 的 MCP 映射改为 `operation_not_supported`，并同步 `docs/mcp-tools.md`、`mapLibraryError` 与对应测试 |
| **永不做** | 核心库网络或模型；HWND 进入 MCP 契约；反作弊模块名单；用完整性高低断言「全部输入消息确定被阻断」；把 `CallerUIAccess == true` 当成自动绕过 UIPI；为探查修改目标消息过滤器（如 `ChangeWindowMessageFilterEx`）；用 `SendMessageTimeout` 成功或像素差证明输入已被消费；默认点击/按键探针；未测量的覆盖率或准确率承诺；`UseBackend(fn)` 临时改全局后端；输入工具内部自动探测；名为 `auto` 的后端值；为让 MCP 工具链看起来完整而先做一个未经样本验证的推荐器 |

本阶段必须：只报告事实、未知与由此导出的风险提示；UIPI 以风险陈述，不写成既成阻断；目标类名只描述目标 HWND 自身；`Input` 的窗口操作与桌面操作分属不同接收类型。

本阶段不做：推荐引擎、画布类名→HID 的自动规则、后代枚举采集、DirectInput 扫描、MCP 服务端任务上下文、MCP 适配层改用 `NewInput`。

## 4. 轻量环境诊断

### 4.1 接口

`ProbeBackend(w *Window) (*BackendProbe, error)`

- `w` 为 nil 或无效 → 既有窗口错误（如 `ErrWindowGone`），无部分结果。
- **不要**复用 `checkReady()`：可见性与是否最小化是输入前置条件。隐藏或最小化窗口仍允许采集静态事实。
- 不发送鼠标键盘消息，不截屏，不遍历 UIA 树，不枚举后代窗口，不枚举进程模块。
- 探查开始时记下目标 HWND、PID、线程 ID 与类名。采集结束前重新检查 HWND 是否仍然有效，以及 PID、线程 ID、类名是否与开始时一致。窗口中途消失，或上述身份不一致（包括 HWND 被销毁后复用），返回 `ErrWindowGone` 或单独的 `ErrTargetChanged`，无部分结果。禁止把不同进程的信息拼成一份完整诊断。不要求首版建立复杂的窗口生命周期监控；取得并握持原进程句柄以防 PID 复用可作为实施时的加强，不是本阶段的开工阻断条件。

`BackendProbe` 本阶段不含后端建议。字段分成观察值与风险提示：

| 字段 | 含义 |
|---|---|
| `HIDStatus` | `initialized` / `driver_unavailable` / `dll_unavailable` / `unknown`。以与 `SetBackend` 共用的内部初始化入口探测，失败写入状态，**不使探针失败**。`initialized` 只表示 `hid.Init()` 成功，不表示系统级输入一定成功，更不表示目标窗口能够正确响应 HID |
| `Features.ProcessName` | 能取到才填，取不到留空，不报假值 |
| `Features.TargetClass` | 目标 HWND 自身的窗口类名；读失败时留空并视为 unknown，不报假值，也不因此使探针失败 |
| `Features.IntegrityRelation` | `target_higher` / `equal` / `lower` / `unknown` |
| `Features.CallerUIAccess` | 三态：能读则填 true/false，读不到为 unknown。实现上使用 `*bool` 或独立三态枚举，不得用单独的 `bool`（无法区分 false 与读不到） |
| `Warnings` | 由观察值导出的风险提示，本阶段：`uipi_likely_blocked`（完整性为 `target_higher` 时）、`integrity_unknown`、`uiaccess_unknown` |

不包含：`SuggestedBackend`、作为推荐含义的 `Confidence`、`standard_child` / `target_standard_control` / `canvas_class` / `dinput_loaded` 等裁断原因。不再使用「仅含原始事实」的 `Reasons` 来存放推导出的风险码；`uipi_likely_blocked` 属于 `Warnings`，不是原始观察。

`uipi_likely_blocked` 表示消息路径**很可能**因完整性隔离受限。`CallerUIAccess` 是解释调用方权限环境的辅助信息，**不代表自动绕过 UIPI**。实际限制仍应结合调用方有效完整性级别、目标完整性级别以及目标窗口的消息过滤策略判断。有 UIAccess 但并非以管理员身份启动、自身未达到与目标同级的高完整性时，仍不能据此取消 `target_higher` 的风险提示。UIAccess 也不能突破系统完整性级别。不得写成「全部输入消息已被阻断」。不得为探查去改目标过滤器。

目标类名只是观察结果。目标是 `Edit`/`Button` 或 `UnityWndClass`/`UnrealWindow`，本阶段都只出现在 `TargetClass` 里，不触发建议。

禁止写入：HWND、token、绝对路径、窗口标题（核心库若将来对 Go 调用方提供标题，须为可选）、完整模块表、截图像素。

### 4.2 HID 初始化入口

在现行 `hid.Init()` 之上抽取一处内部包装，并发安全、可重复调用。供 `ProbeBackend`（写入 `HIDStatus`）以及——若实施——`NewInput(BackendHID)`、`SetBackend` / `checkBackend` 共用错误映射。禁止再写一套互不知情的初始化逻辑，也禁止在 `winput` 里另维护一份与 `hid` 内部状态脱节的「已初始化」标记。

`ProbeBackend` 不执行目标窗口输入操作，但可以初始化本地 HID 运行环境（加载 DLL、初始化 Interception 驱动资源、改变本进程的初始化状态）。因此它不是完全无副作用的静态查询。「只读」若将来用于 MCP，只表示不改变目标应用、不执行输入；调用过程可以有本进程状态变化。

### 4.3 初始化配置冻结与失败重试

`SetHIDLibraryPath` 必须在第一次执行以下任一操作之前调用：

- `ProbeBackend`
- `NewInput(BackendHID)`
- `SetBackend(BackendHID)`

第一次成功初始化后，DLL 路径配置视为冻结，不承诺动态切换。现行实现中，`hid.Init()` 成功后再调用会走幂等快路径；`Load()` 在 DLL 已加载时直接返回，此后更改路径不会生效，除非先关闭并卸载。若将来 `hid` 内部语义变化，仍须遵守本条对外契约。

初始化语义：

- 成功：后续调用快速返回，不重复创建上下文。
- 失败：不得仅因第一次失败就永久缓存失败结果；修复 DLL 路径、安装驱动或恢复环境后，允许再次尝试。现行 `hid.Init()` 只在成功时将 `initialized` 置为真，失败路径会清理资源，已符合本条；实施时不得改成失败也永久缓存的 `sync.Once`。
- 并发初始化不得产生多个相互冲突的初始化过程。

### 4.4 失败政策

- 窗口无效或采集期间目标身份变化：返回错误。
- 完整性读失败：诊断成功，`IntegrityRelation` 为 unknown，可附 `integrity_unknown`。
- UIAccess 读失败：诊断成功，`CallerUIAccess` 为 unknown，附 `uiaccess_unknown`，不并入 `integrity_unknown`。
- HID 初始化失败：写入对应 `HIDStatus`，探针成功。
- 采集失败不得翻译成「应当使用 HID」。

### 4.5 验收（本阶段）

| 项目 | 通过标准 |
|---|---|
| 窗口无效 | 返回窗口错误，无诊断对象 |
| 隐藏或最小化窗口 | 仍可采集（不走 `checkReady`） |
| `target_higher` | 出现 `uipi_likely_blocked`，且不声称全部消息已阻断；不因 `CallerUIAccess == true` 自动取消该提示 |
| 完整性读失败 | `IntegrityRelation` 为 unknown，可附 `integrity_unknown`，不当成 UIPI |
| UIAccess 读失败 | `CallerUIAccess` 为 unknown，附 `uiaccess_unknown` |
| 类名读失败 | `TargetClass` 留空（unknown），探针不失败 |
| HID 驱动/DLL 失败 | 对应 `HIDStatus`，探针不失败 |
| 序列化 | 无 HWND、token、标题、绝对路径 |
| 无建议字段 | 公开结构不含 `SuggestedBackend` |

不验收「建议是否选对后端」。视觉、覆盖率、准确率数字不进入本阶段完成标准。

### 4.6 MCP

本阶段**不**把 `probe_backend` 列为必须交付。若日后出现真实需求：按需调用，返回与核心库相同的事实与警告子集（进程名、目标类名、完整性关系、UIAccess、HID 状态、警告）；默认不返回标题与绝对路径；不返回 HWND；不在 `click` 等工具内自动调用；不加 `auto`。不改变目标应用、不执行输入；允许本进程加载 DLL 或初始化驱动资源。现有 `switch_backend` 仍是调用方显式切换全局后端的入口。

## 5. 任务级 `Input`（独立于探针）

全局 `SetBackend` 修改进程级 `currentBackend`。同一进程中两条 Go 调用路径各需不同后端时会互相覆盖。

```go
type Input struct {
    backend Backend
}

type WindowInput struct {
    input  *Input
    window *Window
}

type DesktopInput struct {
    input *Input
}

func NewInput(backend Backend) (*Input, error)

func (i *Input) ForWindow(w *Window) *WindowInput

func (i *Input) Desktop() *DesktopInput
```

`Input.backend` 创建后不可变。`NewInput` 不依赖 `ProbeBackend`。已知后端的调用方可直接构造。是否立即编码，见 §10，不与诊断捆绑。

Go 不允许同一类型上定义两个同名方法，即使参数不同。因此窗口操作与屏幕/无窗口操作不得都作为 `*Input` 的方法。`WindowInput` 对齐现行 `*Window` 方法名；`DesktopInput` 对齐现行包级屏幕坐标与无窗口方法名。不另造 `TypeText` / `PressKey` / `GlobalType` 之类名字。

```go
input, err := winput.NewInput(winput.BackendHID)
if err != nil {
    return err
}

winInput := input.ForWindow(window)
if err := winInput.Click(100, 100); err != nil {
    return err
}
if err := winInput.Type("123456"); err != nil {
    return err
}

desktop := input.Desktop()
if err := desktop.MoveMouseTo(500, 300); err != nil {
    return err
}
```

`WindowInput` 提供：`Move`、`MoveRel`、`Click`、`ClickRight`、`ClickMiddle`、`DoubleClick`、`Scroll`、`KeyDown`、`KeyUp`、`Press`、`PressHotkey`、`Type`。

`DesktopInput` 提供：`MoveMouseTo`、`ClickMouseAt`、`DoubleClickMouseAt`、`ClickRightMouseAt`、`ClickMiddleMouseAt`、`MouseDown`、`MouseUp`、`ClickMouse`、`MoveMouseBy`、`MoveMouseToExact`、`ScrollWheel`、`PressFor`，以及无窗口的 `KeyDown`、`KeyUp`、`Press`、`PressHotkey`、`Type`。

两套方法虽然部分名字相同，语义按现有实现区分，不能互相替代。例如：

- `WindowInput.Scroll`：Message 为定向窗口滚动；HID 仍返回 `ErrDirectedScrollUnsupported`，不改道为 `ScrollWheel`。
- `DesktopInput.ScrollWheel`：Message 为现行全局滚轮实现（`mouse_event`）；HID 为系统级滚轮输入。

### 5.1 约束（前几轮已按代码核实，此处保留并补充）

- 不采用 `UseBackend(fn)`。公开输入方法自行获取不可重入的 `inputMutex`；在持锁回调中再调 `Window.Click` 等必然死锁。`SetBackend` 只锁 `backendMutex`，持有输入锁也不能隔离全局后端。
- `NewInput` 只接受 `BackendMessage` 与 `BackendHID`。其它整型必须报错，不得解释成 Message。现行 `SetBackend` 对非 HID 整型会直接写入 `currentBackend`，非法值会静默走 Message；收紧 `SetBackend` 不视为本稿已批准的行为变更。
- `NewInput(BackendHID)` 须经 §4.2 的同一初始化入口，成功后才返回对象；失败返回既有错误，不留下半创建实例。构造成功后，各方法不再反复做初始化检查；运行期失败仍返回具体错误。旧 `Window` 路径若改用同一入口，已初始化时只走幂等快路径。自定义 DLL 路径遵守 §4.3。
- 每次调用短持 `inputMutex`，使用对象内后端，不读 `currentBackend`。不在整个 RPA 期间持有该锁。`inputMutex` 只串行单次操作，不独占桌面。
- `NewInput` 解决的是后端配置隔离，不是 RPA 任务之间的输入隔离。任务持有的是后端配置不可变的输入执行器，不提供任务级事务、桌面租约或跨多次调用的独占权。两个 `Input` 仍可能在多次操作之间交错执行。HID 是操作系统级输入，不是某个 `Input` 对象私有的光标和键盘。若调度器需要防止两项业务同时操作同一个桌面，应由上层提供任务锁。
- 创建后 `SetBackend` 不得影响该 `Input`。两个 `Input` 可各用不同后端。
- 现行仅 `moveImpl`、`keyDownImpl`、`keyUpImpl`、`mouseButtonImpl` 接受显式 `Backend`。`Click` / `Type` / `Scroll` / `PressHotkey` 等仍在公开方法内按 `getBackend()` 分支，实施时必须提取，不得假定底层已经全部支持。
- HID 定向滚动仍返回 `ErrDirectedScrollUnsupported`，不改道为 `ScrollWheel`。若 MCP 将来映射此错误，应使用 `operation_not_supported`，而不是沿用今日的 `backend_unavailable`。该项须在未来 MCP 接入阶段同时修改 `docs/mcp-tools.md` 的错误码清单、`internal/mcpadapter/server.go` 的 `mapLibraryError` 以及对应测试；不进入当前轻量诊断的工作范围。今日 `docs/mcp-tools.md` 尚无 `operation_not_supported`，适配器仍将此错误与驱动/DLL 失败一并映射为 `backend_unavailable`。
- Message 下带修饰键的 `PressHotkey` 保留既有前台语义。
- 新旧 API 共存。

### 5.2 与 MCP 的阶段关系

本阶段 `NewInput` 的实施范围不包括 MCP 适配层。现有 MCP `switch_backend` 仍调用 `SetBackend`，输入工具仍走原来的 `Window` 与包级方法，因此仍具有进程级全局效果。单独实现核心库 `NewInput` 不会自动改变该行为，也不得宣称已经解决 MCP 的并发隔离问题。

以后接入 MCP 时，不一定需要服务器端任务上下文；可以在各输入工具调用上携带相同的可选 `backend` 参数，内部再使用任务级 `Input`。那是后续设计，不在本稿实施范围内。

### 5.3 验收（仅在决定实施本节时）

拒绝非法 `Backend`；HID 初始化失败则构造失败；`SetBackend` 不污染已有 `Input`；两对象后端互不干扰；单次操作仍受 `inputMutex` 保护；窗口方法与桌面方法分属 `WindowInput` 与 `DesktopInput`，同一类型上无重名方法。

## 6. 分层与铁律

- 核心库只做本地诊断与输入。禁止 `net`、模型 SDK、MCP 类型进入核心包。
- 诊断结构可序列化。采集失败保留 unknown。
- 不得把全局 `SetBackend` 当作按窗口或按任务的裁决实现。
- 不得用 `UseBackend(fn)` 临时改全局后端。
- 不得在输入方法内部自动调用 `ProbeBackend`。
- 不得在本阶段增加推荐字段来「补全」诊断结果。

## 7. 已知限制

- 本阶段不回答「这项任务该用哪种后端」。
- 完整性为 `target_higher` 只表示风险。`CallerUIAccess` 不能单独抵消该风险；还存在目标消息过滤放行特定消息的可能。
- 目标类名不能证明 Message 会被消费，也不能证明必须 HID。
- HID 状态只说明本进程能否完成 `hid.Init()`，不说明对某个窗口的输入会成功。
- 一次诊断的有效范围是当时的窗口、进程与权限环境；应用重启、窗口重建、换到另一应用或会话变化后应重新采集。HWND 销毁后可能被系统再分配，短时探针以开始/结束时的 HWND、PID、线程 ID、类名复核为务实边界，不能当成完整的窗口生命周期证明。
- 任务级 `Input` 不独占桌面，也不隔离 HID 光标与键盘。

## 8. 反过度设计复核

- 拒绝本阶段实现静态规则推荐引擎：证据不足以证明它能可靠代替调用方结合任务约束做选择。
- 拒绝用后代控件、画布白名单、DirectInput 模块去「提高建议完整度」。
- 拒绝默认微探针、像素预言机、反作弊名单、无测量的准确率承诺。
- 拒绝为 MCP 工具链完整而先交一个推荐器。
- 拒绝把 `NewInput` 写成探针的附属功能，也拒绝因暂缓推荐而否定其隔离价值。
- 拒绝为架构完整而强制 `ProbeBackend` 与 `NewInput` 同时交付。

## 9. 恢复推荐引擎的前提（以后做）

只有在真实样本上证明「静态建议能显著减少选择后端的试错」之后，才考虑恢复 `SuggestedBackend`。建议先取 5～10 个现有 RPA 目标，记录进程与窗口、实际结构、Message/HID 是否真正完成业务操作（文本写入、按钮触发、滚动发生等，而不是 API 成功）、任务是否允许抢前台，再对照若使用推荐规则会给出什么结果。统计：有明确更优后端的样本中建议正确的比例、返回未定的比例、建议与任务约束冲突的比例。这不是机器学习数据集，只回答规则有没有足够收益。

若恢复，必须遵守既有约束（§12），不得回退到「后代标准控件 ⇒ 父窗口高置信 Message」或「完整性更高 ⇒ 全部消息确定被阻断」，也不得把 UIAccess 写成自动绕过 UIPI。

## 10. 实施顺序

三项分开判断，不按原 PR1（完整探针）→ PR2（`Input`）→ PR3（MCP 推荐）整包推进。`ProbeBackend` 与 `NewInput` 并非必须同时交付：前者解决诊断需求，后者解决多后端任务的状态隔离。若当前运行仍是单进程、单任务、固定后端，`NewInput` 可以作为独立设计保留，不必为了架构完整而立即实施。

1. **轻量诊断**（§4）：可单独编码，不改现有输入行为，不产生实际输入（但可能初始化本进程 HID 环境）。
2. **`NewInput`**（§5）：独立评估时机。动机是同进程多后端的 Go API 隔离。不依赖诊断是否已经落地。本阶段不包括 MCP 适配层。
3. **MCP `probe_backend`**：有调用需求再做，只暴露事实与警告。
4. **MCP 输入工具改用任务级 `Input`**：另行设计；现有 `switch_backend` 在此之前保持进程级全局效果。
5. **推荐引擎**：仅在 §9 的样本结论支持时再开设计修订。
6. **`operation_not_supported` 与 MCP 错误目录同步**：留待 MCP 阶段，与映射及测试一起改。

本文件在用户确认前不得视为已批准开工的编码任务。

## 11. 变更说明

- 相对 `e2579e8`：闭合 `Input` / `WindowInput` / `DesktopInput` 的类型与方法归属；统一 HID 初始化的副作用、路径冻结与失败重试契约，并将 `HIDStatus` 的成功态定为 `initialized`；修正 UIAccess 不能自动绕过 UIPI 的表述，改用三态并拆分 `Features` / `Warnings`；写明本阶段 `NewInput` 不改变 MCP；区分 `BackendMessage` 在窗口 API 与全局 API 中的语义；补充窗口身份复核与类名读取失败的处理；将 MCP 错误码同步列为后续约束。
- 相对 `330821a`：删除本阶段的 `SuggestedBackend` / `decideBackend` / 推荐用置信度；后代枚举与画布自动规则移出首版；`probe_backend` 改为按需；明确最佳后端不单由窗口特征决定；`NewInput` 从探针实施链中拆出，约束原文保留。
- 相对初稿 `d4ce95a`：此前已删除 `UseBackend(fn)`、纠正 UIPI 绝对措辞与 `standard_child` 误判等，这些纠正仍然有效。

## 12. 若恢复推荐时必须遵守的既有结论

- `SuggestedBackend` 不得复用 `type Backend int`（零值是 `BackendMessage`）。
- 仅当目标 HWND **自身**类名命中标准控件白名单时，才能把标准控件当作 Message 的结构证据；后代 `Edit`/`Button` 不得使父窗口获得高置信 Message。
- UIPI：完整性优先于控件类型；表述为 `likely_blocked` / `uipi_likely_blocked`；`Confidence` 若恢复，只表示规则依据强弱，不是成功率。`CallerUIAccess` 不得写成自动绕过。
- HID 不就绪时不得强行建议 HID。
- MCP 默认不返回标题与绝对路径；定向滚动错误不得再与驱动不可用共用 `backend_unavailable`。
