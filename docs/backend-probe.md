# 后端环境诊断与任务级输入（Backend Probe 缩减稿）

- 基础设计：`docs/mcp-tools.md`（适配层目标、非目标、分层、工具契约、兼容策略）
- 分层与约束：见本稿 §6、§8 和 `docs/mcp-tools.md`。与适配层相关的约定须同时符合本稿与该适配层契约
- 关联文档：`API_CN.md`、`API.md`、`docs/mcp-tools.md`。现状依据：`probe.go`、`input.go`、`paste.go`、`winput.go`、`window/clipboard.go`、`hid/control.go`、`hid/interception/interception.go`。历史缺口设计可在提交 `2e4f3e5` 中查阅，不作为当前文件路径引用
- 本稿取代提交 `330821a` 所载的完整推荐引擎方案：那一版的技术约束（UIPI 措辞、目标 HWND 与后代控件的区分、`UseBackend` 不可行、`Backend` 零值冲突等）在恢复推荐功能时仍须遵守，见 §12
- 术语：`ProbeTarget`、`ProbeHID`、`IntegrityRelation`、`CallerUIAccess`、`Input` 以本稿 §4、§5 定义为准

> 本轮状态：**已实施并提交（`943d17c`）**。基线为 `9ff95cbb0a28572dbaa8048362e36609153fe72c`。本轮将目标进程句柄贯穿采集升级为正式要求，并新增 `DesktopInput.PasteByClipboard`；设计与实现均以本稿为准。旧实现符合原契约。实施结果与未执行项见 §10，验收表见 §4.6 / §5.3。

## 1. 背景与一句话目标

`winput` 以窗口为中心，在 `BackendMessage` 与 `BackendHID`（Interception 系统级输入，通常需前台并移动光标）之间切换。二者不是简单的优劣，而是不同机制。

`BackendMessage` 在窗口目标 API 中主要采用窗口消息机制（一定条件下可后台、不移动真实光标）。对于屏幕坐标和无窗口 API，则依照各自实现使用对应的 Win32 输入机制，并非一律 `PostMessage`：例如现行 `MoveMouseTo` 使用 `SetCursorPos`，全局点击路径使用 `mouse_event`，无窗口 `Type` 在非 HID 时使用 `SendInput` 的 Unicode 输入。现有 Go API 同时提供全局 `SetBackend` 和任务级 `NewInput`；MCP 侧仍由敏感工具 `switch_backend` 切换全局后端。

调用方实际要回答三件事：

1. 目标应用能否有效处理 Message？
2. 当前环境能否使用 HID？
3. 当前任务需要哪种输入语义（可否抢前台、要不要定向滚动、可靠性与后台约束如何权衡）？

Windows 没有可靠的静态接口，可以证明任意 HWND 会消费某种输入消息。`PostMessage` 成功也不等于目标处理了输入。因此第 3 问不能由窗口静态特征单独决定：最佳后端是应用、操作类型与任务约束的函数，不是窗口特征的函数。例如定向 `Window.Scroll` 在 HID 上返回 `ErrDirectedScrollUnsupported`；Message 下带 Ctrl/Alt/Shift 的 `PressHotkey` 会走 `pressHotkeyForeground`，可能切换前台。窗口门面与桌面门面即使方法名相近，底层语义也不相同，不能互相替代。

一句话目标：**`ProbeTarget` 报告完整性关系、调用方 UIAccess、目标窗口自身类名与进程名；`ProbeHID` 报告本进程 HID 初始化结果。二者都不推荐后端，也都不修改全局后端。任务级 `Input` 是独立的状态隔离能力，不依赖探针。**

原则：能够明确检测的，给出事实；无法明确检测的，保留未知。已通过实际验证的应用，由调用方直接配置后端。不得为了消灭「未知」而叠加启发式规则。不得将诊断包装成输入兼容性的确定性证明，也不得把探针写成每条 RPA 的必经步骤。

## 2. 现状能力矩阵（审计结果）

| 能力 | 现状 | 相对本设计 |
|---|---|---|
| `SetBackend` / `switch_backend` | 进程级全局后端；HID 初始化失败有明确错误 | 已有；诊断不得改写它 |
| `ErrPermissionDenied` | 注释指向 UIPI | 已有；诊断可在投递前报告完整性隔离风险 |
| `FindChildByClass` | `FindWindowExW`，只找直接子窗口 | 发现子控件后应对该子 HWND 本身采集类名，不得把后代标准控件当成父窗口的输入证明 |
| `Window.Scroll` + HID | `ErrDirectedScrollUnsupported` | 操作类型会影响后端选择；诊断不应据此「建议改 HID 以完成滚动」 |
| `Window.PressHotkey` + Message | 含修饰键时可能切前台 | Message 不等于全部操作都能后台完成 |
| 任务级、不可变的后端执行器 | 已有 `Input` / `WindowInput` / `DesktopInput` | 本轮仅扩充桌面粘贴与对应验收（§5），不依赖探针 |
| 环境事实诊断（完整性、类名、HID 状态） | 已有分开的 `ProbeTarget` / `ProbeHID` | 本轮升级目标进程句柄生命周期（§4.1），不新增总入口 |
| 目标进程句柄 | 当前实现由 `ProbeTarget` 在起始快照后打开一次，名称与 Token 经同一句柄读取，打开后与结束各核对一次身份；PID / TID 均须非零 | 已实施：句柄贯穿最终核对，增加打开后核对，失败按 unknown 降级 |
| 剪贴板粘贴 | 包级 `PasteByClipboard` 使用全局后端；`DesktopInput.PasteByClipboard` 使用固定后端，共享同一私有 helper | 已实施：保留包级兼容与既有格式限制 |
| 由静态规则产出 `SuggestedBackend` | 无 | **本阶段不做**；待真实样本证明收益后再议（§9） |
| MCP 观察类诊断工具 | 无 | **按需**，不作为默认工具链的必经步（§4.7） |

## 3. 范围矩阵

| | 内容 |
|---|---|
| **现在做（已实施）** | ① `ProbeTarget`：完整性关系、调用方 UIAccess、目标 HWND 自身类名、进程名，以及由此导出的风险提示；不调用 `hid.Init`（§4.1）；② `ProbeHID`：五个 HID 状态；允许预热本进程驱动，不修改全局后端（§4.2）；③ 任务级 `Input` 的桌面粘贴扩展、完整方法表及资源边界（§5）。实施与未执行项见 §10 |
| **以后做** | 用 5～10 个真实应用、以业务结果（而非 API 返回成功）验证之后，再考虑恢复 `SuggestedBackend` 裁决引擎；后代窗口类名作为可选结构信息（不参与父窗口后端判断）；DirectInput 模块枚举；MCP 上的目标探针与 HID 探针（有真实使用需求时，默认仍只返回事实）；MCP 输入工具可选 `backend` 并内部改用任务级 `Input`（与核心库 `NewInput` 分阶段，见 §5.2、§10）；有限深度 UIA、特征缓存、Sampling；将 `ErrDirectedScrollUnsupported` 的 MCP 映射改为 `operation_not_supported`，并同步 `docs/mcp-tools.md`、`mapLibraryError` 与对应测试 |
| **永不做** | 核心库网络或模型；HWND 进入 MCP 契约；反作弊模块名单；用完整性高低断言「全部输入消息确定被阻断」；把 `CallerUIAccess == enabled` 当成自动绕过 UIPI；为探查修改目标消息过滤器（如 `ChangeWindowMessageFilterEx`）；用 `SendMessageTimeout` 成功或像素差证明输入已被消费；默认点击/按键探针；未测量的覆盖率或准确率承诺；`UseBackend(fn)` 临时改全局后端；输入工具内部自动探测；名为 `auto` 的后端值；为让 MCP 工具链看起来完整而先做一个未经样本验证的推荐器；会调用 `hid.Init` 的总入口 `ProbeBackend(w)` |

本阶段必须：只报告事实、未知与由此导出的风险提示；UIPI 以风险陈述，不写成既成阻断；目标类名只描述目标 HWND 自身；`Input` 的窗口操作与桌面操作分属不同接收类型；窗口事实与 HID 预热是两次调用。

本阶段不做：推荐引擎、画布类名→HID 的自动规则、后代枚举采集、DirectInput 扫描、MCP 服务端任务上下文、MCP 适配层改用 `NewInput`、把两个探针重新合成一个总会初始化 HID 的入口。

## 4. 轻量环境诊断

### 4.1 目标探针

`ProbeTarget(w *Window) (*TargetProbe, error)`

- `w` 为 nil 或无效 → `ErrWindowGone`，无部分结果。
- **不要**复用 `checkReady()`。可见性与最小化是输入前置条件。`Window.IsVisible` 同时要求窗口可见且未最小化，因此隐藏窗口和最小化窗口都会被 `checkReady` 拒绝。诊断仍允许采集这两类窗口的静态事实。
- 不发送鼠标键盘消息，不截屏，不遍历 UIA 树，不枚举后代窗口，不枚举进程模块，**不调用 `hid.Init`**，不冻结 DLL 路径。
- 固定本次 `hwnd := w.HWND`，后续不重读可能被调用方修改的 `w.HWND`。起始快照 `start` 记录有效 HWND、同一次取得的非零 PID / 线程 ID，以及可读类名和 known 标记。HWND 无效、PID / TID 不可读或为 0，返回 `ErrWindowGone`，无部分结果、不打开进程。
- 起始身份有效后尽早尝试一次 `OpenProcess(start.pid)`，沿用 `PROCESS_QUERY_LIMITED_INFORMATION`，不扩大权限、不在采集后续阶段重新按 PID 打开。
- **成功取得的目标进程句柄必须贯穿进程事实采集和最终 HWND 身份核对。** 打开后立即安排 `defer` 或等价统一清理，各成功/错误路径恰好关闭一次；目标 Token 句柄也须正确关闭。
- 打开成功后读取 `afterOpen`，执行 `resolveIdentity(start, afterOpen)` 或等价完整规则（见下表）。失败立即返回既有身份错误，无 report。不得用 `afterOpen` 替换原始 `start`，也不得提升中间读到的类名为新比较基准。
- 目标进程名经同一目标进程句柄读取；目标 Token 经同一句柄调用 `OpenProcessToken`，再读取完整性 RID。完整性比较的是当前进程令牌与目标进程令牌；调用方完整性及 UIAccess 仍从调用方进程独立读取。调用方采集顺序可调，但不能破坏句柄生命周期与身份核对顺序。结果不放原始完整性级别。
- 采集后读取 `end`，执行 `resolveIdentity(start, end)`。成功才按原始 start/end 规则填写 `TargetClass`、归一化 unknown 并生成 report；最后释放目标进程句柄。禁止拼接不同进程的信息。
- `OpenProcess` 失败不自动使整体失败：起始/结束身份稳定时，进程名留空、完整性关系为 `unknown`，调用方 UIAccess 独立读取。不重试打开，不推断 UIPI 已阻断、不建议 HID、不调用 `ProbeHID`。此路径不强制执行成功打开后的 `afterOpen` 检查；若额外读取快照，仍沿用同一错误规则。
- 已开句柄后名称或 Token 查询失败，只降级受影响事实；例如 Token 失败不清空已取得的名称。任一身份核对失败则丢弃全部事实，返回 `ErrWindowGone` / `ErrTargetChanged`，无部分 report（§4.5）。
- 不获取 `inputMutex`。不建立窗口生命周期监控。读取 PID 到 `OpenProcess` 不是原子操作；尽早打开和打开后复核只缩小竞态、检测可观察变化，不消除全部 TOCTOU。持有已打开进程对象可防该对象销毁导致 PID 复用，不阻止进程退出、不保证 HWND 稳定；打开前已发生复用的竞态仍不能完全排除。

身份与类名的契约（中间与最终核对均以原始 `start` 为基准）：

| 情况 | 结果 |
|---|---|
| 起始、打开后或结束快照 HWND 无效，或 PID / 线程 ID 不可读、为 0 | `ErrWindowGone`，无探针对象；起始失败不打开进程，后续失败关闭已取得句柄 |
| 本次 HWND 仍有效，PID 或线程 ID 与 start 不同 | `ErrTargetChanged`，无探针对象 |
| start 类名已知，本次类名可读且不同 | `ErrTargetChanged`，无探针对象 |
| start 类名已知，本次类名读失败，其他身份一致 | 不因类名读取失败单独报错；最终 `TargetClass` 使用 start 值 |
| start 类名未知，afterOpen 类名可读 | 不提升比较资格、不替换 start；最终仍按 start/end 填写 |
| start 类名未知、end 可读，其他身份一致 | 探针成功；`TargetClass` 使用 end 值 |
| start 与 end 类名均未知，其他身份一致 | 探针成功；`TargetClass` 留空 |

类名读到空串算已知值，按字符串比较和填写；各次快照使用同一种读取。start 已知时，成功结果使用 start 值；未知时仅使用可读的 end 值，不用中间值补成身份证据。打开后核对通过仍不能省略最终核对。

`TargetProbe` 不含后端建议，也不含 HID 状态。字段分成观察值与风险提示：

| 字段 | 含义 |
|---|---|
| `Features.ProcessName` | 目标进程可执行文件的文件名。能取到才填，取不到留空，不报假值，不写绝对路径 |
| `Features.TargetClass` | 目标 HWND 自身的窗口类名。填写规则见上表。它是观察结果，不是身份证据 |
| `Features.IntegrityRelation` | `target_higher` / `equal` / `target_lower` / `unknown`。从调用方看目标。零值与空串都按 `unknown` 理解 |
| `Features.CallerUIAccess` | `enabled` / `disabled` / `unknown`。与完整性关系、`HIDStatus` 一样是字符串取值。Go 零值和空串都按 `unknown` 理解。探针不返回空串。不得用 `bool`、`*bool` 或以 `iota` 把有意义的状态放在零值上 |
| `Warnings` | 由观察值导出的风险提示，本阶段只有三条，见下 |

警告的闭合规则：

| 条件 | 警告 |
|---|---|
| `IntegrityRelation == target_higher` | `uipi_likely_blocked` |
| `IntegrityRelation == unknown` | `integrity_unknown` |
| `CallerUIAccess == unknown` | `uiaccess_unknown` |

`target_higher` 与 `unknown` 不会同时出现，因此 `uipi_likely_blocked` 与 `integrity_unknown` 也不会同时出现。`uipi_likely_blocked` 可以和 `uiaccess_unknown` 同时出现。`equal` 与 `target_lower` 不产生完整性警告。`CallerUIAccess == enabled` 不取消 `uipi_likely_blocked`。UIAccess 读失败不并入 `integrity_unknown`。警告按这个固定顺序输出，缺哪条就跳过哪条：`uipi_likely_blocked`、`integrity_unknown`、`uiaccess_unknown`。

`uipi_likely_blocked` 表示消息路径**很可能**因完整性隔离受限。`CallerUIAccess` 是解释调用方权限环境的辅助信息，**不代表自动绕过 UIPI**。实际限制仍应结合调用方有效完整性级别、目标完整性级别以及目标窗口的消息过滤策略判断。有 UIAccess 但并非以管理员身份启动、自身未达到与目标同级的高完整性时，仍不能据此取消 `target_higher` 的风险提示。UIAccess 也不能突破系统完整性级别。不得写成「全部输入消息已被阻断」。不得为探查去改目标过滤器。只存比较关系，不存原始完整性级别。

目标类名只是观察结果。目标是 `Edit`/`Button` 或 `UnityWndClass`/`UnrealWindow`，本阶段都只出现在 `TargetClass` 里，不触发建议。

禁止写入：HWND、token、绝对路径、窗口标题（核心库若将来对 Go 调用方提供标题，须为可选）、完整模块表、截图像素、HID 状态。

### 4.2 HID 探针

`ProbeHID() HIDStatus`

- 不接收窗口，也不读取 `currentBackend`。不获取 `inputMutex`。
- 走 §4.3 的同一初始化入口。可以加载 DLL、初始化 Interception 驱动资源、把本进程的初始化状态变成已初始化。
- 不修改全局后端。
- 初始化失败仍返回状态，这次调用成功，不返回 error。
- 成功值使用 `initialized`：只表示这一次初始化入口返回成功，不表示系统级输入一定成功，更不表示某个窗口能够正确响应 HID。
- 不返回 HWND、token、标题、绝对路径、原始错误文本。调用方能看到的信号就是状态本身。
- 已初始化后再调用，走初始化入口的幂等快路径，不创建第二份上下文，也不重新扫描设备。
- 零值与空串都是 `unknown`，不是 `initialized`。

| 初始化入口的结果 | `HIDStatus` |
|---|---|
| 返回成功 | `initialized` |
| 库找不到，或库已加载但必要符号缺失 | `dll_unavailable` |
| 驱动上下文打不开（`hid.ErrDriverNotInstalled`） | `driver_unavailable` |
| 上下文曾经建立，但鼠标和键盘设备都没有，随后卸载 | `devices_unavailable` |
| 其余错误 | `unknown` |

现行实现里，至少有一个鼠标或键盘设备时初始化就会成功。`ProbeHID` 不另加「两种设备都必须存在」的条件。实现依据见 `hid/control.go`。

归类只用 `errors.Is`，按下表顺序（实现见 `probe.go`）：

| 顺序 | 哨兵 | `HIDStatus` |
|---|---|---|
| 入口返回 nil | — | `initialized` |
| 1 | `hid.ErrDriverNotInstalled` | `driver_unavailable` |
| 2 | `hid.ErrDevicesUnavailable` | `devices_unavailable` |
| 3 | `hid.ErrLibraryNotFound` 或 `hid.ErrSymbolsMissing` | `dll_unavailable` |
| 其余 | — | `unknown` |

`hid.ErrLibraryNotFound` 是 `interception.ErrLibraryNotFound` 的别名，`Load` 继续用 `%w` 包着操作系统的加载错误返回。`hid.ErrSymbolsMissing` 是新哨兵，符号缺失并释放库之后返回。`hid.ErrDevicesUnavailable` 是新哨兵，上下文销毁并卸载之后返回。这三个都从 `hid` 导出，供 `ProbeHID` 识别。`winput` 不把它们重新公开。不匹配错误文案。

`ProbeHID` 不是完全无副作用的静态查询。「只读」若将来用于 MCP，只表示不改变目标应用、不执行输入、不修改全局后端。

### 4.3 初始化入口

在现行 `hid.Init()` 之上保持一处初始化入口，并发安全、可重复调用。底层已有 `initMutex`，`hid/interception` 的路径设置、加载、卸载已有 `libraryMu`；不在 winput 增加第二把初始化锁，不改为 `sync.Once`，不改底层初始化机制。供 `ProbeHID`、`NewInput(BackendHID)`、`SetBackend` / `checkBackend` 使用。`ProbeTarget` 不使用它。禁止再写一套互不知情的初始化逻辑，也禁止在 `winput` 里另维护一份与 `hid` 内部状态脱节的「已初始化」标记。

公开结果分两路翻译（实现见 `winput.go` 的 `translateHIDInit`）：

| 初始化入口的结果 | `ProbeHID` | `SetBackend`、`checkBackend`、`NewInput(BackendHID)` |
|---|---|---|
| 成功 | `initialized` | 成功。`SetBackend` 改写全局后端；`NewInput` 返回实例 |
| `hid.ErrDriverNotInstalled` | `driver_unavailable` | `ErrDriverNotInstalled` |
| 库找不到、符号缺失、没有设备、其它失败 | 按 §4.2 的状态 | 包成 `ErrDLLLoadFailed` |

`ProbeHID` 失败不修改全局后端。`NewInput` 失败不留下实例。没有设备时 `NewInput` 得到 `ErrDLLLoadFailed` 是沿用今天的包装，不是 `devices_unavailable` 写错。本阶段不收紧 `SetBackend`。哨兵替换旧文案之后，`errors.Is(..., ErrDLLLoadFailed)` 仍然成立；包在里面的英文文本可以改变，调用方不得依赖 `"no interception devices found"` 或 `"library loaded but symbols missing"`。

### 4.4 初始化配置冻结与失败重试

`SetHIDLibraryPath` 必须在第一次成功执行以下任一操作之前调用：

- `ProbeHID`
- `NewInput(BackendHID)`
- `SetBackend(BackendHID)`

`ProbeTarget` 不在此列。第一次成功初始化后，DLL 路径配置视为冻结，不承诺动态切换。现行实现中，`hid.Init()` 成功后再调用会走幂等快路径；`Load()` 在 DLL 已加载时直接返回，此后更改路径不会生效。`winput` 没有公开卸载入口（`hid.Close` 只在子包，不是官方生命周期 API）。若将来 `hid` 内部语义变化，仍须遵守本条对外契约。

初始化语义：

- 成功：后续调用快速返回，不重复创建上下文。上下文留到进程结束。
- 失败：不得仅因第一次失败就永久缓存失败结果；修复 DLL 路径、安装驱动或恢复环境后，允许再次尝试。现行 `hid.Init()` 只在成功时将 `initialized` 置为真，失败路径会清理资源，已符合本条；实施时不得改成失败也永久缓存的 `sync.Once`。
- 并发初始化不得产生多个相互冲突的初始化过程。

### 4.5 失败政策

- 起始、打开后或结束 HWND 无效，或 PID / TID 不可读、为 0：`ErrWindowGone`，无部分结果。
- 身份不一致：`ErrTargetChanged`，无部分结果。打开后及结束核对均遵守 §4.1；类名只有在原始 start 已知、且本次可读并不同才构成不一致。
- `OpenProcess` 失败：身份稳定时探针仍可成功，进程名空、完整性关系 `unknown`，附 `integrity_unknown`；调用方 UIAccess 独立读取，不重试打开。
- 名称或目标 Token 查询失败：只降级受影响事实，不清空独立成功取得的事实；完整性任一侧不可读则关系 `unknown`，附 `integrity_unknown`。
- UIAccess 读失败：`CallerUIAccess` 为 `unknown`，附 `uiaccess_unknown`，不并入 `integrity_unknown`。
- 后续身份核对优先于部分事实：发现消失或变化则丢弃事实，无 report；所有已成功取得的句柄仍须恰好关闭一次。
- `ProbeHID` 初始化失败：返回对应 `HIDStatus`，不推荐后端。
- `ProbeTarget` 的失败或降级不表示 UIPI 已阻断，不得据此推荐 HID 或调用 `ProbeHID`。

### 4.6 验收（本阶段）

| 项目 | 通过标准 |
|---|---|
| 窗口无效 | `ProbeTarget` 返回 `ErrWindowGone`，无诊断对象 |
| 隐藏或最小化窗口 | `ProbeTarget` 仍可采集（不走 `checkReady`） |
| 打开后或结束时 PID、线程 ID 变化，或 start 已读类名与本次可读类名不同 | `ErrTargetChanged`，无诊断对象 |
| 结束时类名读失败，身份其余部分一致 | 探针成功；已有开始值则 `TargetClass` 用开始值 |
| 仅结束时读到类名 | 探针成功；`TargetClass` 用结束值；不返回 `ErrTargetChanged` |
| 两端类名都读不到 | 探针成功；`TargetClass` 留空 |
| 任一身份快照取不到 PID / 线程 ID，或 PID / TID 为 0 | `ErrWindowGone`，无诊断对象 |
| 关系、类名、进程名、UIAccess 都未知，身份稳定 | `ProbeTarget` 仍然成功 |
| `target_higher` | 出现 `uipi_likely_blocked`，且不声称全部消息已阻断；不因 `CallerUIAccess == enabled` 取消该提示 |
| 完整性读失败 | `IntegrityRelation` 为 `unknown`，附 `integrity_unknown`，不当成 UIPI |
| UIAccess 读失败 | `CallerUIAccess` 为 `unknown`，附 `uiaccess_unknown` |
| `ProbeTarget` 与 HID | 不调用 `hid.Init`，不改变初始化状态，不冻结 DLL 路径 |
| `hid.ErrLibraryNotFound` 或 `hid.ErrSymbolsMissing` | `dll_unavailable`；`SetBackend` / `NewInput(BackendHID)` 仍是 `ErrDLLLoadFailed` |
| `hid.ErrDevicesUnavailable` | `devices_unavailable`；上述入口仍是 `ErrDLLLoadFailed`，且不留下 `Input` |
| 驱动上下文打不开 | `driver_unavailable`；上述入口仍是 `ErrDriverNotInstalled` |
| 其余初始化错误 | `unknown`；上述入口仍是 `ErrDLLLoadFailed` |
| 归类方式 | 表测 `errors.Is` 的顺序。不匹配错误文案。不启动驱动也能测 |
| `ProbeHID` 任一失败 | 调用本身成功，不修改 `currentBackend` |
| 只有鼠标或只有键盘 | `initialized`。不额外要求两种设备都在 |
| 序列化 | 无 HWND、token、标题、绝对路径；`ProcessName` 只是文件名 |
| 无建议字段 | 公开结构不含 `SuggestedBackend`，也没有 `ProbeBackend` |
| 取值零值 | `IntegrityRelation`、`CallerUIAccess`、`HIDStatus` 的零值都是 `unknown` |
| 警告同时出现 | `target_higher` 且 UIAccess 读失败时，同时有 `uipi_likely_blocked` 与 `uiaccess_unknown`，顺序固定 |
| 固定 HWND 与打开时机 | 起始有效后尽早按 start.pid 打开一次；后续不重读 w.HWND、不重新按 PID 打开 |
| 成功路径顺序 | start → OpenProcess → afterOpen 核对 → 同句柄事实采集 → end 核对 → report → 关闭；调用方 Token 顺序可调但不破坏不变量 |
| 同对象采集及清理 | 记录名称和目标 Token 使用同一进程句柄；覆盖成功、查询失败、中间/结束身份失败，已开进程句柄恰好关闭一次，Token 也正确关闭 |
| 打开后 HWND 无效或 PID/TID 不可读、为 0 | ErrWindowGone，无 report，不继续事实采集，关闭一次 |
| 中间类名规则 | 类名读取失败不单独失败；已知空串变成其他值返回 ErrTargetChanged；start 未知、中间已知、end 又不同不提升比较资格 |
| OpenProcess 失败、身份稳定 | 名称空、关系 unknown、调用方 UIAccess 独立；不重开、不推断 UIPI、不建议 HID，仍最终核对 |
| 进程事实分别读取失败 | 名称失败不丢其他事实；Token 失败保留名称；身份失败仍无 report |
| TOCTOU 边界 | 不宣称已消除打开前竞态、进程退出或 HWND 复用风险 |
| NewInput(HID) 直接测试 | 直接调用构造器，成功有效；驱动失败 nil + ErrDriverNotInstalled，库/符号/设备/其他初始化失败 nil + ErrDLLLoadFailed；用 errors.Is 分类，失败不改 currentBackend |
| 非法 Backend | NewInput 返回 nil + ErrInvalidBackend，初始化调用 0 次，全局不变；不收紧 SetBackend |

不验收「建议是否选对后端」。视觉、覆盖率、准确率数字不进入本阶段完成标准。警告列表由已经得到的完整性关系和 UIAccess 导出，HID 状态由哨兵导出；这两段用取值做表测，不依赖真实窗口或驱动。采集令牌、类名和进程名另测。句柄、后端及错误路径优先用窄接口或现有可替换 seam 做确定性测试，不依赖真实 PID 恰好复用。

### 4.7 MCP

本阶段**不**把诊断工具列为必须交付。若日后出现真实需求：按需调用，分别返回与 `ProbeTarget`、`ProbeHID` 相同的事实与警告。默认不返回标题与绝对路径；不返回 HWND；不在 `click` 等工具内自动调用；不加 `auto`。不得恢复一个总会初始化 HID 的总入口。合并成一个 MCP 工具时，必须写明它会执行 `ProbeHID` 的进程内副作用，且仍然不修改全局后端。现有 `switch_backend` 仍是调用方显式切换全局后端的入口。

## 5. 任务级 `Input`（独立于探针）

全局 `SetBackend` 修改进程级 `currentBackend`。同一进程中两条 Go 调用路径各需不同后端时会互相覆盖。

```go
type Input struct {
    backend Backend
    // 未导出的有效标记只由 NewInput 成功路径打开。
    // 零值没有这层关联。
}

type WindowInput struct {
    input  *Input
    window *Window
}

type DesktopInput struct {
    input *Input
}

func NewInput(backend Backend) (*Input, error)

func (i *Input) ForWindow(w *Window) (*WindowInput, error)

func (i *Input) Desktop() (*DesktopInput, error)
```

`Input.backend` 创建后不可变。`NewInput` 不依赖 `ProbeTarget` 或 `ProbeHID`。已知后端的调用方可直接构造。是否立即编码，见 §10，不与诊断捆绑。

`ForWindow` 与 `Desktop` 返回 `(*T, error)`，因为零值 `Input` 要在这一层返回 `ErrInputInvalid`，而合法输入上的 `ForWindow(nil)` 又不能在这一层失败。见下列有效性约束。

```go
input, err := winput.NewInput(winput.BackendHID)
if err != nil {
    return err
}

winInput, err := input.ForWindow(window)
if err != nil {
    return err
}
if err := winInput.Click(100, 100); err != nil {
    return err
}
if err := winInput.Type("123456"); err != nil {
    return err
}

desktop, err := input.Desktop()
if err != nil {
    return err
}
if err := desktop.MoveMouseTo(500, 300); err != nil {
    return err
}
```

方法集合闭合如下；`PasteByClipboard` 为本轮新增，且仅属于 `DesktopInput`。

| 序号 | WindowInput（12 项） | DesktopInput（18 项） |
|---|---|---|
| 1 | `Move` | `MoveMouseTo` |
| 2 | `MoveRel` | `ClickMouseAt` |
| 3 | `Click` | `DoubleClickMouseAt` |
| 4 | `ClickRight` | `ClickRightMouseAt` |
| 5 | `ClickMiddle` | `ClickMiddleMouseAt` |
| 6 | `DoubleClick` | `MouseDown` |
| 7 | `Scroll` | `MouseUp` |
| 8 | `KeyDown` | `ClickMouse` |
| 9 | `KeyUp` | `MoveMouseBy` |
| 10 | `Press` | `MoveMouseToExact` |
| 11 | `PressHotkey` | `ScrollWheel` |
| 12 | `Type` | `PressFor` |
| 13 | — | `KeyDown` |
| 14 | — | `KeyUp` |
| 15 | — | `Press` |
| 16 | — | `PressHotkey` |
| 17 | — | `Type` |
| 18 | — | `PasteByClipboard` |

```go
func (d *DesktopInput) PasteByClipboard(text string) error

// 保留包级兼容入口。
func PasteByClipboard(text string) error
```

两套方法虽然部分名字相同，语义按现有实现区分，不能互相替代。例如：

- `WindowInput.Scroll`：Message 为定向窗口滚动；HID 仍返回 `ErrDirectedScrollUnsupported`，不改道为 `ScrollWheel`。
- `DesktopInput.ScrollWheel`：Message 为现行全局滚轮实现（`mouse_event`）；HID 为系统级滚轮输入。

### 5.1 约束（保留既有契约，纳入本轮扩展）

- 不采用 `UseBackend(fn)`。公开输入方法自行获取不可重入的 `inputMutex`；在持锁回调中再调 `Window.Click` 等必然死锁。`SetBackend` 只锁 `backendMutex`，持有输入锁也不能隔离全局后端。
- `NewInput` 只接受 `BackendMessage` 与 `BackendHID`。`BackendMessage` 的值是 0，它合法，而且不初始化 HID。其它整型，包括负数，在调用初始化入口之前返回 `ErrInvalidBackend`（`invalid input backend`）：不留下实例，不加载 DLL，不冻结库路径。不得解释成 Message，不得复用 `ErrBackendUnavailable`，也不得返回无法用 `errors.Is` 识别的普通错误。现行 `SetBackend` 对非 HID 整型会直接写入 `currentBackend`，非法值会静默走 Message；收紧 `SetBackend` 不在本轮范围（实现见 `winput.go`）。
- 零值 `Input` 不是 `BackendMessage`。`NewInput` 成功才建立有效关联。`var in winput.Input`、nil `*Input` 上的 `ForWindow` 和 `Desktop` 返回 nil 和 `ErrInputInvalid`，不返回一个无效对象。任何输入方法在零值或 nil 接收者上返回 `ErrInputInvalid`，不 panic。
- `WindowInput` 持有调用方传入的 `*Window`，不另做 HWND 快照。调用时读当时的 `HWND`，与今天的窗口方法相同。
- `ForWindow(nil)` 在接收者是合法 `Input` 时返回非 nil 的 `WindowInput` 和 nil error。之后调用该对象的方法返回 `ErrWindowGone`。`ForWindow` 不在这一层验证非 nil 窗口是否仍存在；那是各方法的既有窗口检查。
- 零值 `WindowInput`、`DesktopInput`，以及不是由合法 `Input.ForWindow` / `Desktop` 得到的操作面，调用时返回 `ErrInputInvalid`。nil 接收者同样返回 `ErrInputInvalid`，不 panic。只有「合法 `Input` 上 `ForWindow(nil)`」这一条路径在调用时返回 `ErrWindowGone`。
- `NewInput(BackendHID)` 须经 §4.3 的同一初始化入口，成功后才返回对象；失败返回既有错误，不留下半创建实例。构造成功后，固定后端选择与门面包装层不新增初始化检查；`ready()` 只验证接收者，`acquireBackend(fixed)` 的非 nil 路径不调用 `getBackend`、`checkBackend` 或 `ensureHID`。底层 HID 运行期 `EnsureInit` 等原行为保持，不承诺整条输入链绝不触发底层初始化；运行期失败仍返回具体错误。旧 `Window` 路径若改用同一入口，已初始化时只走幂等快路径。自定义 DLL 路径遵守 §4.4。`NewInput` 的失败是 error，不是 `HIDStatus`；不要把它和 `ProbeHID` 合成一种返回习惯。
- 每次调用短持 `inputMutex`，使用对象内后端，不读 `currentBackend`。不在整个 RPA 期间持有该锁。`inputMutex` 只串行单次操作，不独占桌面。
- `NewInput` 解决的是后端配置隔离，不是 RPA 任务之间的输入隔离。任务持有的是后端配置不可变的输入执行器，不提供任务级事务、桌面租约或跨多次调用的独占权。两个 `Input` 仍可能在多次操作之间交错执行。HID 是操作系统级输入，不是某个 `Input` 对象私有的光标和键盘。若调度器需要防止两项业务同时操作同一个桌面，应由上层提供任务锁。
- 创建后 `SetBackend` 不得影响该 `Input`。两个 `Input` 可各用不同后端。
- 现有门面已将固定后端传入内部 helper（见 `input.go`、`winput.go`）。本轮仍须逐项核对完整 12/18 方法的实际调用链，不能仅凭传了 `backend()` 就断言隔离成立；不得新增回读全局后端的旁路或在持锁期间重入公开输入方法。
- HID 定向滚动仍返回 `ErrDirectedScrollUnsupported`，不改道为 `ScrollWheel`。若 MCP 将来映射此错误，应使用 `operation_not_supported`，而不是沿用今日的 `backend_unavailable`。该项须在未来 MCP 接入阶段同时修改 `docs/mcp-tools.md` 的错误码清单、`internal/mcpadapter/server.go` 的 `mapLibraryError` 以及对应测试；不进入当前轻量诊断的工作范围。今日 `docs/mcp-tools.md` 尚无 `operation_not_supported`，适配器仍将此错误与驱动/DLL 失败一并映射为 `backend_unavailable`。
- Message 下带修饰键的 `PressHotkey` 保留既有前台语义。
- 新旧 API 共存。`DesktopInput.PasteByClipboard` 使用对象固定后端；包级 `PasteByClipboard` 保持全局默认后端选择，签名不变。窗口定向粘贴不在范围内；`GetCursorPos`、窗口查找、文本读取、DPI 等查询仍留在现有包级函数或 `Window`，不进入 Input。

#### 粘贴流程与错误优先级

两入口共用私有粘贴 helper；包级入口传 nil 表示全局后端，新门面先 `d.ready()` 再取 `d.backend()`，传入固定后端。

1. 门面首先验证接收者，先于文本校验、backend 解引用和加锁。nil / 零值门面即使 text 为空也返回 `ErrInputInvalid`，不取 backend、不操作剪贴板、不 panic。
2. 合法门面及包级入口校验文本：空串返回 `ErrEmptyPasteText`，空白合法。失败不获取 inputMutex、不打开剪贴板、不发键。
3. 获取 `inputMutex`，按 fixed/global 解析本次 `cb`。全局解析失败时不打开、不修改剪贴板。fixed 不新增 HID 可用性预检，不能保证运行期错误发生在修改剪贴板之前。
4. 调用 `window.SwapUnicodeText(text)`；仅 `swapErr == nil` 才用 `pressHotkeyWith(cb, KeyCtrl, KeyV)` 或等价显式后端 helper 发键。两入口取得 cb 后均复用该值，不重新选择后端，不调用 `pressHotkeyLocked`。
5. 只要 `restore != nil`，均尝试恢复一次，包括 swap 失败或按键失败。swap 失败不得发键，也不得直接返回而跳过恢复。固定 HID 的运行期按键失败可能已在 swap 之后，仍按此规则恢复。
6. 保持 `errors.Join(swapErr, keyErr, restoreErr)`，未执行阶段对应 nil；保留各错误的 `errors.Is` 可识别性。不把所有 HID 运行期错误统一转换为 `ErrSendInputFailed`。
7. swap、发键、restore 全部处于同一 `inputMutex` 临界区，不递归获取锁或调用会加锁的公开输入方法。

#### 包级后端兼容与并发边界

包级函数仍按全局默认后端机制解析，不承诺与并发 `SetBackend` 原子一致、特定执行顺序或调用级串行。`inputMutex` 不阻止 `SetBackend`。无需保留旧实现多次读取全局的具体竞态细节；解析出 cb 后，本次 Ctrl+V 使用同一 cb。现有 `acquireBackend(nil)` 先 `checkBackend()` 再 `getBackend()`，内部两次读取不是原子验证/执行快照，不得扩大承诺或为此改变 SetBackend 语义。

#### 剪贴板与桌面资源边界

两个入口都只是临时替换 Unicode 文本并尝试恢复原 Unicode 文本，**不是完整剪贴板备份与恢复**。现有机制见 `paste.go`、`window/clipboard.go`；本轮不重写格式保存机制。

| 原剪贴板内容 | 恢复范围 |
|---|---|
| 可读取的 Unicode 文本 | 恢复保存的 Unicode 文本内容 |
| Unicode 文本加图片、HTML、文件列表或自定义格式 | 只恢复 Unicode 文本；其他格式不恢复 |
| 没有可读取的 Unicode 文本（例如只有图片） | 写回空 Unicode 缓冲，其他格式不恢复 |
| CF_TEXT | Windows 可能提供合成的 CF_UNICODETEXT，但不保证原 CF_TEXT 或原格式集合恢复 |

写入调用 `EmptyClipboard`，会清空现有格式；恢复只写回 `CF_UNICODETEXT`。保留待粘贴文本换行规范化与恢复已读文本的既有行为。

固定后端不固定前台窗口。Ctrl+V 送往执行时当前前台，不自动切换前台、不接收 HWND、不提供窗口定向粘贴。`inputMutex` 仅协调遵守该锁的本进程调用，不阻止其他进程或其他剪贴板使用者。事件排队不等于目标已消费，立即恢复仍有既有时序限制；nil 不证明粘贴成功，非 nil 也不证明按键未发送。不承诺前台隔离、跨进程剪贴板独占或任务级事务。

### 5.2 与 MCP 的阶段关系

本阶段 `NewInput` 的实施范围不包括 MCP 适配层。现有 MCP `switch_backend` 仍调用 `SetBackend`，输入工具仍走原来的 `Window` 与包级方法，因此仍具有进程级全局效果。单独实现核心库 `NewInput` 不会自动改变该行为，也不得宣称已经解决 MCP 的并发隔离问题。

以后接入 MCP 时，不一定需要服务器端任务上下文；可以在各输入工具调用上携带相同的可选 `backend` 参数，内部再使用任务级 `Input`。那是后续设计，不在本稿实施范围内。

### 5.3 验收（实施后核对）

| 项目 | 通过标准 |
|---|---|
| NewInput(Message) | 有效实例，不初始化 HID |
| 非法 Backend（含负数） | nil + ErrInvalidBackend，初始化调用 0 次，全局不变；SetBackend 兼容不收紧 |
| NewInput(HID) 直接构造测试 | 成功返回有效 HID Input；驱动失败 nil + ErrDriverNotInstalled；库/符号/设备/其他初始化失败 nil + ErrDLLLoadFailed；errors.Is 分类，失败不改 currentBackend；不得只测试错误翻译 helper |
| nil / 零值 Input | ForWindow / Desktop 返回 nil + ErrInputInvalid，不 panic |
| nil / 零值操作面 | 所有输入方法返回 ErrInputInvalid，不 panic |
| 合法 Input 上 ForWindow(nil) | 返回非 nil 门面；方法调用 ErrWindowGone，保留既有其他窗口检查 |
| Paste 接收者优先级 | nil / 零值加空串仍 ErrInputInvalid；先 ready 后文本/后端解引用，不取锁、不动剪贴板 |
| Paste 文本 | 合法门面或包级入口空串 ErrEmptyPasteText，不取锁、不打开剪贴板、不发键；空白合法 |
| 完整方法集合 | Window 12 项、Desktop 18 项与 §5 一致；Desktop 有 Paste、Window 无 Paste，查询仍禁入 |
| 现有方法集合负向测试 | 已更新为完整 12/18 集合断言：Desktop 有 Paste、Window 无 Paste，查询仍禁入 |
| 全方法双向隔离 | 固定 HID + 全局 Message 仍走 HID；固定 Message + 全局 HID 仍走 Message；12/18 每方法均有调用链及覆盖记录，可表测或使用 seam，无须每项真实桌面输入 |
| 包装层初始化边界 | 固定 Message 操作不走全局 HID 初始化；固定 HID 构造后操作不新增 winput 包装层初始化检查；计数区分构造、SetBackend 设置、底层原运行期行为 |
| 包级兼容与 cb | 包级签名及全局默认选择不变；解析 cb 后发键用同一 cb；不测试未承诺的并发原子顺序，不混淆 acquireBackend(nil) 的两次读取 |
| 全局解析失败 | 不修改剪贴板；fixed 不新增预检，也不承诺 HID 运行期失败先于 swap |
| swap 失败且 restore 为 nil | 不发键、不恢复，保留 swap 错误 |
| swap 失败且 restore 非 nil | 不发键，恢复一次；保留 swap 及 restore 错误 |
| 按键失败或恢复失败 | 有 restore 均调用一次；保留按键错误、恢复错误；成功返回 nil 亦不证明目标消费 |
| errors.Join 兼容 | 注入 Message 发送 ErrSendInputFailed 与恢复 ErrClipboardFailed，errors.Is 分别成立；其他 HID 运行期错误按原类型保留，不泛化该发送哨兵 |
| 单次临界区 | swap/key/restore 顺序正确且受同一 inputMutex 保护，无递归锁；不独占整个任务或桌面 |
| 既有输入语义 | SetBackend 不污染已有 Input，两对象后端互不干扰；HID 定向 Scroll 仍 ErrDirectedScrollUnsupported；Message 修饰键热键前台语义不变 |
| GoDoc 与 API 文档 | 已同步 API.md / API_CN.md 与 GoDoc，说明 global/fixed、ready 优先级、Unicode-only/格式转换、前台/消费时序和失败恢复 |

优先利用现有 seam 或窄接口做确定性测试，保留已有 ProbeTarget 不初始化 HID 的测试。本轮未新增 HID 锁、初始化缓存或重复同义测试来替代实际构造失败验收。Windows 专属完整测试、race、vet、真实剪贴板与 Interception 冒烟须在受控 Windows 环境另行验证；当前环境的结果与未执行项见 §10。

## 6. 分层与铁律

- 核心库只做本地诊断与输入。禁止 `net`、模型 SDK、MCP 类型进入核心包。
- 诊断结构可序列化。采集失败保留 unknown。
- 不得把全局 `SetBackend` 当作按窗口或按任务的裁决实现。
- 不得用 `UseBackend(fn)` 临时改全局后端。
- 不得在输入方法内部自动调用 `ProbeTarget` 或 `ProbeHID`。
- 不得在本阶段增加推荐字段来「补全」诊断结果。
- 不得提供 `ProbeBackend(w)`，也不得让 `ProbeTarget` 调用 `hid.Init`。

## 7. 已知限制

- 本阶段不回答「这项任务该用哪种后端」。
- 完整性为 `target_higher` 只表示风险。`CallerUIAccess` 不能单独抵消该风险；还存在目标消息过滤放行特定消息的可能。
- 目标类名不能证明 Message 会被消费，也不能证明必须 HID。只读到一端时，它只是观察结果。
- `ProbeHID` 只说明本进程能否完成初始化，不说明对某个窗口的输入会成功。成功会把驱动上下文留到进程结束。第一次成功之后不再重新扫描设备。只有鼠标或只有键盘时，结果仍是 `initialized`。
- `SetBackend` 与 `NewInput(BackendHID)` 对「没有设备」和「符号缺失」仍返回 `ErrDLLLoadFailed`。要区分这两种情况，看 `ProbeHID` 的状态。
- 一次 `ProbeTarget` 只描述当时窗口、进程与权限环境。应用重启、窗口重建、换应用或会话变化后应重新采集。持句柄及 start/afterOpen/end 核对以 §4.1 为边界，不消除读 PID 至打开之间的 TOCTOU、不阻止进程退出，也不是完整窗口生命周期证明。
- 任务级 Input 只隔离后端配置，不独占桌面、前台、HID 光标或键盘。新增 DesktopInput.PasteByClipboard 使用固定后端，但仍共享剪贴板；包级 PasteByClipboard 和其他旧全局输入方法不受已有 Input 配置约束。
- 粘贴只恢复 CF_UNICODETEXT，EmptyClipboard 清空的其他格式不恢复；无可读 Unicode 则恢复空 Unicode。系统可合成 CF_TEXT→Unicode，不保证原格式集合。inputMutex 不提供跨进程独占；排队不等于消费，恢复时序、前台变化和并发 SetBackend 的限制见 §5.1。

## 8. 反过度设计复核

- 拒绝本阶段实现静态规则推荐引擎：证据不足以证明它能可靠代替调用方结合任务约束做选择。
- 拒绝用后代控件、画布白名单、DirectInput 模块去「提高建议完整度」。
- 拒绝默认微探针、像素预言机、反作弊名单、无测量的准确率承诺。
- 拒绝为 MCP 工具链完整而先交一个推荐器。
- 拒绝把 `NewInput` 写成探针的附属功能，也拒绝因暂缓推荐而否定其隔离价值。
- 拒绝为架构完整而强制 `ProbeTarget`、`ProbeHID` 与 `NewInput` 三者同时交付。`ProbeTarget` 与 `ProbeHID` 同属诊断设计，仍是两次调用。
- 拒绝用一个总会初始化 HID 的总入口把 §4.1 和 §4.2 重新绑在一起。
- 拒绝用错误文案识别 HID 状态，也拒绝把 `hid` 的新哨兵再从 `winput` 公开出去。
- 拒绝用 ErrBackendUnavailable 表示非法 Backend。DesktopInput.PasteByClipboard 是经本轮修订纳入的输入能力，不为方法表对称而扩充 WindowInput、Input 本体或查询函数。拒绝据此承诺完整剪贴板恢复、前台隔离或桌面独占。

## 9. 恢复推荐引擎的前提（以后做）

只有在真实样本上证明「静态建议能显著减少选择后端的试错」之后，才考虑恢复 `SuggestedBackend`。建议先取 5～10 个现有 RPA 目标，记录进程与窗口、实际结构、Message/HID 是否真正完成业务操作（文本写入、按钮触发、滚动发生等，而不是 API 成功）、任务是否允许抢前台，再对照若使用推荐规则会给出什么结果。统计：有明确更优后端的样本中建议正确的比例、返回未定的比例、建议与任务约束冲突的比例。这不是机器学习数据集，只回答规则有没有足够收益。

若恢复，必须遵守既有约束（§12），不得回退到「后代标准控件 ⇒ 父窗口高置信 Message」或「完整性更高 ⇒ 全部消息确定被阻断」，也不得把 UIAccess 写成自动绕过 UIPI。

## 10. 实施顺序

本轮实施已完成并提交（`943d17c`）。§4.1 持句柄要求与 §5 桌面粘贴已落地；旧实现符合原契约。实施结果与未执行项见下。

按可独立评审的范围推进：

1. **目标进程句柄**：同句柄名称/Token、打开后与结束身份核对、独立降级和清理；不改 HID 或粘贴实现。已完成。
2. **桌面粘贴与输入覆盖**：新增固定后端方法、复用包级兼容 helper、错误恢复与资源边界；补 NewInput(HID) 直接测试和完整 12/18 调用链覆盖；同步 GoDoc、API.md、API_CN.md，不重构 HID 初始化机制。已完成。
3. **验证回报**：运行环境允许的单元测试、race、vet；在受控 Windows 环境验证专属路径及必要冒烟。明确未运行项目，不以静态审阅代替通过结论。`go test ./...` 已通过；`go test -race ./...` 在本机启动时以 `0xc0000139` 失败，未能执行测试体；`go vet ./...` 报告既有 unsafe.Pointer 告警（`uia/uia.go`、`probe.go` Token 完整性转换），本次未新增 unsafe 代码。真实剪贴板与 Interception 驱动冒烟未执行。

诊断与任务输入仍是独立能力，不要求为了架构完整重新交付已有 ProbeHID / NewInput，也不要求同时推进后续功能。ProbeTarget 仍不初始化 HID；ProbeHID 仍使用 §4.3 的同一入口，不修改全局后端。

以下仍是未来另行设计范围：MCP 诊断工具、MCP 输入接入任务级 Input、§9 推荐引擎，以及 operation_not_supported 与 MCP 错误目录同步。不得借本轮变更调整 MCP 或恢复推荐机制。

## 11. 变更说明

- 本轮修订（已实施并提交，`943d17c`）：将目标进程句柄贯穿采集与最终核对升级为正式要求，补完整中间身份/类名和 PID0/TID0 规则、失败降级与清理验收（§4.1 / §4.5 / §4.6）；将 PasteByClipboard 纳入 DesktopInput，保留包级 global、新门面 fixed，补 ready 优先级、初始化职责、cb 并发边界、Unicode-only 格式/资源限制、异常恢复及完整 12/18 验收（§5 / §7 / §8）。现有底层 HID 同步与运行期行为不变；新增构造失败直接测试要求，不新增锁或 Once，不改 MCP / SetBackend。
- 本轮引用修正：移除无法验证的 ADR、术语及原则文件引用，改用本稿章节和存在源码；历史决定保留，下列旧 Paste 排除决定标为已被替代。
- 此前评审：`NewInput` 对非法 `Backend` 返回 `ErrInvalidBackend`，且发生在 HID 初始化之前（§5.1）。`BackendMessage` 合法。`SetBackend` 仍不收紧。当时决定“PasteByClipboard 留在全局后端上，不进入 DesktopInput”——**已被本轮修订替代（superseded）**：包级兼容保留，DesktopInput 新增固定后端方法；该历史句不是当前排除规则。
- 上一轮：HID 状态用哨兵和 `errors.Is` 识别。`hid.ErrSymbolsMissing` 与 `hid.ErrDevicesUnavailable` 为新哨兵，`hid.ErrLibraryNotFound` 别名已有的库错误。哨兵从 `hid` 导出，`winput` 不重新公开（§4.2）。`ErrDLLLoadFailed` 的 `errors.Is` 结果保持不变，旧英文文案不再作为契约。
- 上一轮：`HIDStatus` 闭合为五个词，零值是 `unknown`（§4.2）。初始化入口仍只有一处；`ProbeHID` 使用这五个词，`SetBackend` / `checkBackend` / `NewInput(BackendHID)` 保持今天的公开错误（§4.3）。
- 上一轮：按已确认的选择拆成 `ProbeTarget` 与 `ProbeHID`，删除会初始化 HID 的 `ProbeBackend` 总入口（§4.1 / §4.2）。`ErrTargetChanged` 与类名「观察 / 身份」规则按起始/结束读写结果闭合（§4.1；本轮增设中间核对，仍保持原 start/end 填写规则）。完整性关系与 UIAccess 使用显式取值，零值为 `unknown`（§4.1）。零值 `Input` 与未关联的操作面返回 `ErrInputInvalid`；`ForWindow` / `Desktop` 改为返回 error（§5.1）。当时稿头曾增加原则文件引用；该引用目前无法验证，本轮改为本稿 §6 / §8 与存在的适配层文档，不将其作为有效依据。
- 相对 `e2579e8`：闭合 `Input` / `WindowInput` / `DesktopInput` 的类型与方法归属；统一 HID 初始化的副作用、路径冻结与失败重试契约，并将 `HIDStatus` 的成功态定为 `initialized`；修正 UIAccess 不能自动绕过 UIPI 的表述，改用三态并拆分 `Features` / `Warnings`；写明本阶段 `NewInput` 不改变 MCP；区分 `BackendMessage` 在窗口 API 与全局 API 中的语义；补充窗口身份复核与类名读取失败的处理；将 MCP 错误码同步列为后续约束。
- 相对 `330821a`：删除本阶段的 `SuggestedBackend` / `decideBackend` / 推荐用置信度；后代枚举与画布自动规则移出首版；`probe_backend` 改为按需；明确最佳后端不单由窗口特征决定；`NewInput` 从探针实施链中拆出，约束原文保留。
- 相对初稿 `d4ce95a`：此前已删除 `UseBackend(fn)`、纠正 UIPI 绝对措辞与 `standard_child` 误判等，这些纠正仍然有效。

## 12. 若恢复推荐时必须遵守的既有结论

- `SuggestedBackend` 不得复用 `type Backend int`（零值是 `BackendMessage`）。
- 仅当目标 HWND **自身**类名命中标准控件白名单时，才能把标准控件当作 Message 的结构证据；后代 `Edit`/`Button` 不得使父窗口获得高置信 Message。
- UIPI：完整性优先于控件类型；表述为 `likely_blocked` / `uipi_likely_blocked`；`Confidence` 若恢复，只表示规则依据强弱，不是成功率。`CallerUIAccess` 不得写成自动绕过。
- HID 不就绪时不得强行建议 HID。
- MCP 默认不返回标题与绝对路径；定向滚动错误不得再与驱动不可用共用 `backend_unavailable`。
