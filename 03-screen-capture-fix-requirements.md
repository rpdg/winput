# 03 屏幕截图修复需求（screen capture）

## 1. 背景与基线

基线：commit `dd234a5`。`screen/capture.go`、`window/dpi.go` 与 `6ec5407` 相同；`window/proc.go` 多了一行 `GetAncestor`；`internal/mcpadapter/server.go` 有改动。文中行号仅供参考，以内容定位为准。外部审查已逐条对照代码和 Microsoft Learn 核实，证据见 `scratch/winput-review/verify-capture-review.md`。采纳：缺少 GdiFlush、CaptureRegion 先截全屏再裁剪。部分采纳：Session 0（改为补错误码和文档）。驳回：逐屏截取拼接、worker 池/SIMD、WTSGetActiveConsoleSessionId 检查。另补入审查遗漏的问题：ReleaseDC 跨线程、内存上限只算一份缓冲、PreserveAlpha、线程 DPI 上下文、DPI 函数名错误。本文只覆盖截图，不含其他审查的 BUG-001～004。仓库没有 `screen/dpi.go`，DPI 代码在 `window/dpi.go`。

## 2. 修复项

### CAP-01 截图 GDI 序列锁定 OS 线程，补 GdiFlush（P2）

- **位置**：`screen/capture.go` 的 `CaptureVirtualDesktopWithOptions`（约 :89-154）；`window/proc.go` 的 GDI 段（约 :67-74），在此新增 `GdiFlush`。
- **现状**：
  - 从 `GetDC(0)` 到 defer 的 `ReleaseDC`，中间没有 `runtime.LockOSThread`。goroutine 可能在任意 syscall 或异步抢占点换到别的 OS 线程，串行转换和并行转换（`wg.Wait()`）两条路径都会。
  - 没有 GdiFlush。ReleaseDC、恢复 SelectObject、DeleteObject、DeleteDC 的返回值都不检查。
  - `hBitmap != 0 && ppvBits == nil` 时漏了 DeleteObject。
- **依据**：
  - [GetDC](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdc) / [ReleaseDC](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-releasedc)：“ReleaseDC must be called from the same thread that called GetDC.”
  - [CreateDIBSection](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createdibsection)：访问位图数据前须调用 GdiFlush。
  - [GdiFlush](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-gdiflush)：只刷新调用线程的批次；返回值反映的是整个批次。
- **要求**：
  1. **锁定范围。** 函数入口（参数校验之后）第一件事是 `runtime.LockOSThread()`，紧接着注册 `defer runtime.UnlockOSThread()`。这个 defer 最先注册，所以最后执行。之后注册的各项释放会先于解锁执行。从 DPI 检查（CAP-05）、读取虚拟屏（CAP-02）、全部 GDI 调用、像素转换直到全部释放，都在这一个锁定区内完成。不允许“先释放屏幕 DC、解锁后再转换”的写法。
  2. **清理残留批次。** 加锁后先调用一次 GdiFlush，忽略结果，避免把这个线程之前遗留的批处理错误算到本次截图头上。
  3. **调用顺序。** GetDC → CreateCompatibleDC → CreateDIBSection → SelectObject → BitBlt → GdiFlush → 转换（串行或并行均在锁内）→ 恢复 SelectObject → DeleteObject → DeleteDC → ReleaseDC。
  4. **截图失败判定。** BitBlt 返回 0，或 BitBlt 之后的 GdiFlush 返回 0，都算截图失败，不返回图像。BitBlt 已失败时，不再调用 GdiFlush。GoDoc 写明：GdiFlush 报告的是调用线程整个批次的结果。
  5. **释放必须走完。** 无论截图成功还是失败，已创建的对象都要按第 3 步的逆序全部释放，不允许中途 return 漏掉任何一步。恢复 SelectObject、DeleteObject、DeleteDC、ReleaseDC 任何一个返回 0，都算一个清理错误。
  6. **返回规则只有一种。**
     - 截图成功、清理无错：返回 `(img, nil)`。
     - 截图成功、有清理错误：返回 `(img, errors.Join(清理错误...))`，GoDoc 写明此时图像有效。
     - 截图失败：返回 `(nil, errors.Join(原始错误, 清理错误...))`。
  7. CreateDIBSection 返回非 0 句柄但 `ppvBits == nil` 时，先 DeleteObject，再按截图失败处理。
- **不做/边界**：不调用 `SetThreadDpiAwarenessContext`。不加全局锁，并发截图各自锁定自己的线程。
- **验收**：代码审查确认锁定范围、defer 注册顺序和释放顺序；Windows 实机 CAP-T1。

### CAP-02 只截请求区域，虚拟屏只读一次（P2）

- **位置**：`screen/capture.go` 的 `CaptureRegion`（约 :236-274）和 `CaptureVirtualDesktopWithOptions`（约 :65-76）。
- **现状**：
  - CaptureRegion 先截全屏，再读一次原点（:249-250）后裁剪，原点前后读了两次。
  - `x - vx32` 是 int32 运算。
  - 内存上限按全屏判断。
  - 没有带 Options 的版本。
- **要求**：
  1. `w <= 0 || h <= 0` 在加锁和 DPI 检查之前返回错误（保持现有行为）。
  2. 新增内部函数，全屏和区域两条路径共用。锁内顺序：线程 DPI 检查 → 一次读取 4 个 `GetSystemMetrics(SM_*VIRTUALSCREEN)` → 求交 → 内存检查 → GDI 序列。全屏路径的请求矩形，就是这次读到的虚拟屏矩形。锁外不预读，也不调用 `VirtualBounds`（它会再读一遍）。
  3. 求交和内存计算全部用 int64。交集为空时返回错误。DIB 按交集尺寸创建。BitBlt 的目标是 (0,0)，源坐标是交集左上角，按有符号 32 位传入，负值不钳成 0。
  4. DIB 保持 32 位、`BI_RGB`、负的 `biHeight`（自顶向下）；转换按这个布局读取。
  5. 新增 `func CaptureRegionWithOptions(x, y, w, h int32, opts CaptureOptions) (*image.RGBA, error)`。`CaptureRegion` 签名不变，等价于传默认选项。
  6. 返回值语义：区域部分越界时返回交集；图像 `Rect` 从 (0,0) 开始，(0,0) 对应交集的左上角。
- **不做/边界**：不改变返回类型，不增加“实际原点”这个返回值。
- **验收**：CAP-U1、CAP-U2；Windows 实机 CAP-T2。

### CAP-03 内存上限按峰值计算（P2，已定：方案 A）

- **位置**：`screen/capture.go` 的 `CaptureOptions`、`defaultOptions`、内存检查（约 :40-49、:78-87）和 `convertToRGBA`（约 :162）；测试 `winput_test.go` 的 `CaptureWithOptions` 子测试（约 :543-567）。
- **要求**：
  1. 峰值 = `2 × w × h × 4`（DIB 加输出缓冲），w、h 取交集尺寸，用 int64 计算。
  2. 上限字节数 = `limitMB × 1024 × 1024`。`MaxMemoryMB <= 0` 时用默认值 1000。
  3. 峰值或上限的乘法会溢出 int64 时，返回错误，不允许回绕后放行。
  4. 峰值大于上限，或大于 `math.MaxInt`（386）时，返回错误。错误文本中的 MB 是峰值，向上取整，和上限同一单位，例如 `resolution too large: 11520x2160 requires 190 MB peak (limit: 100 MB)`。
  5. `MaxMemoryMB` 的 GoDoc 改为“峰值上限，默认 1000”。变更说明写明：显式传值的调用方，可截像素数减半。
  6. 改 `CaptureWithOptions` 测试：按峰值（两份缓冲）判断是否超过 100MB。三块 4K 横排（11520×2160）单份 99,532,800 B，小于 100 MiB；峰值 199,065,600 B，超过 100 MiB。如果不改，新代码下这个测试会因为截图报错走 `t.Skipf`，问题被掩盖。
- **验收**：CAP-U3。

### CAP-04 错误带系统错误码；文档写明不支持的环境（P3）

- **位置**：`screen/capture.go` 中各处 `fmt.Errorf("... failed")`。
- **要求**：
  1. 抽一个纯函数，例如 `apiError(name string, e error) error`：`e` 是非 0 的 `syscall.Errno` 时，返回 `fmt.Errorf("%s failed: %d: %w", name, uint32(errno), errno)`，文本形如 `GetDC failed: 5: Access is denied.`；Errno 为 0 或不是 Errno 时，返回 `fmt.Errorf("%s failed", name)`，不包装。
  2. GetDC、CreateCompatibleDC、CreateDIBSection、SelectObject、BitBlt、GdiFlush 以及 CAP-01 的各项清理失败，都通过这个函数生成错误；`e` 取 `LazyProc.Call` 的第三个返回值。
  3. 单元测试只测这个纯函数：非 0 Errno 时 `errors.Is(err, syscall.Errno(x))` 成立、文本带数字码；为 0 时不包装。不为测试增加可替换的 Win32 调用。
  4. GoDoc 和 API 文档写明：服务进程或 Session 0、锁屏、安全桌面、最小化的 RDP 会话不受支持，可能报错，也可能返回全黑图且不报错；库不检测黑图。
- **不做/边界**：不加会话预检。以后如确需预检，用 `ProcessIdToSessionId` 或窗口站 `WSF_VISIBLE`（[Interactive Services](https://learn.microsoft.com/en-us/windows/win32/services/interactive-services)）。
- **验收**：单元测试同第 3 点；实机表现记录在 CAP-T4。

### CAP-05 DPI 检查看调用线程；修正函数名（P3）

- **位置**：`screen/capture.go`（约 :61-63）；`window/dpi.go` 的 `IsPerMonitorDPIAware`（约 :102-142）；`window/proc.go` 的 `ProcGetProcessDpiAwarenessCtx`（约 :47）。
- **要求**：
  1. 在锁定区内检查调用线程。`GetThreadDpiAwarenessContext` 和 `GetAwarenessFromDpiAwarenessContext` 两个函数都存在时，要求 `GetAwarenessFromDpiAwarenessContext(GetThreadDpiAwarenessContext()) == DPI_AWARENESS_PER_MONITOR_AWARE (2)`。PMv1 和 PMv2 都映射到 2，都算通过。两个函数都存在而结果不等于 2 时，直接失败，不再回退到进程级检查。只有函数不存在时，才使用现有的 `IsPerMonitorDPIAware()`。
  2. 错误文本都必须包含子串 `not Per-Monitor DPI Aware`（`docs/mcp-tools.md` 第 183 行附近的验收用到）：
     - 线程不满足、进程也不满足（默认进程的常见情况）：沿用现有文本 `process is not Per-Monitor DPI Aware; call winput.EnablePerMonitorDPI() first`。
     - 线程不满足、进程满足：`calling thread is not Per-Monitor DPI Aware although the process is; restore this thread's DPI awareness context`。
     - 走回退路径时：沿用现有文本。
  3. `proc.go` 改为加载 `GetDpiAwarenessContextForProcess`（Win10 1803+，NULL 表示当前进程，现有 `Call(0)` 不变），`dpi.go` 注释同步更正版本号。`IsPerMonitorDPIAware` 保持进程级语义，签名不变（`cmd/mcp-server/main.go` 依赖它）。
- **依据**：[High DPI](https://learn.microsoft.com/en-us/windows/win32/hidpi/high-dpi-desktop-application-development-on-windows)（虚拟化看线程上下文）；[DPI_AWARENESS](https://learn.microsoft.com/en-us/windows/win32/api/windef/ne-windef-dpi_awareness)（只有 -1/0/1/2 四个值，PMv2 没有单独的值）；[GetThreadDpiAwarenessContext](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getthreaddpiawarenesscontext)；[GetAwarenessFromDpiAwarenessContext](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getawarenessfromdpiawarenesscontext)；[GetDpiAwarenessContextForProcess](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdpiawarenesscontextforprocess)。
- **验收**：Windows 实机 CAP-T5；在 Win10 1803+ 上确认新函数名 `Find()` 成功。

### CAP-06 PreserveAlpha 弃用（P3，已定）

- **位置**：`screen/capture.go` 的 `CaptureOptions`、`convertBGRAtoRGBASerial`（约 :40-49、:199-203）。
- **要求**：保留字段，GoDoc 标 `Deprecated:`，写明该字段不再生效；无论取值，输出的 A 一律为 255。
- **验收**：单元测试断言 `PreserveAlpha` 为 true 和 false 时，A 都全为 255；CAP-T6 只作留档。

### CAP-07 可选：CAPTUREBLT 开关（P3，先测后定）

- **要求**：先做 CAP-T7。只有当 SRCCOPY 截不到分层窗口时，才在 `CaptureOptions` 增加 `IncludeLayered bool`（默认 false；为 true 时 ROP 用 `SRCCOPY | CAPTUREBLT (0x40000000)`），GoDoc 写明可能导致鼠标闪烁、截图变慢。依据：[BitBlt](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-bitblt)。

### CAP-08 转换细节（P3）

- **位置**：`screen/capture.go`（约 :185-234）。
- **要求**：
  1. 并行份数用 `runtime.GOMAXPROCS(0)`。
  2. `convertBGRAtoRGBASerial` 遇到空切片直接返回。
  3. 保留 1MB 阈值和每帧临时起 goroutine 的做法。
  4. 转换在 CAP-01 的锁定区内执行。
- **验收**：CAP-U4。

### 待决（不作为本次需求）

- MCP `capture_screen`（`internal/mcpadapter/server.go` 约 :387-393）每次截一张全屏，只为返回宽高。改用虚拟屏尺寸更省，但会失去“截图功能是否可用”的探测意义，需要负责人决定（约定见 `docs/mcp-tools.md:29`）。

## 3. 明确不做

- 逐屏截取再拼接：Per-Monitor 进程看到的就是各屏物理像素，公共 DC 默认 MM_TEXT，等尺寸 BitBlt 不会缩放。
- worker 池或 SIMD：起 8 个 goroutine 约 1.8 µs，1080p 并行转换约 0.4 ms，开销可以忽略。
- `WTSGetActiveConsoleSessionId() > 0` 检查：这个函数返回的是物理控制台的会话，不是本进程的会话，检查不出 Session 0。

## 4. 测试与验收

### 4.1 单元测试（Windows 上运行；CI 至少执行 `GOOS=windows go vet ./screen ./window` 和 `GOOS=windows GOARCH=386 go test -c ./screen`）

求交、内存计算、错误生成都抽成纯函数，不为测试增加可替换的 Win32 调用。

- **CAP-U1** 求交：完全在内、跨左上、跨右下、负坐标副屏、完全在外（报错）；交集左上角为负时保持负值。
- **CAP-U2** int64：`x = MaxInt32`、`x = MinInt32`、`x + w` 超出 int32 时不回绕。
- **CAP-U3** 内存：峰值等于上限时通过，多 1 字节时报错；`MaxMemoryMB <= 0` 时取默认 1000；`limitMB` 极大或 `w × h` 极大导致乘法溢出时报错；超过 `math.MaxInt` 时报错；错误文本中的 MB 为峰值、向上取整。
- **CAP-U4** 转换：串行与并行结果逐字节一致（1MB+4、1080p、末块不整除）；空切片不 panic；A 恒为 255。
- **CAP-U5** 错误生成函数：见 CAP-04 第 3 点。
- 改 `winput_test.go` 的 `CaptureWithOptions`：见 CAP-03 第 6 点。

### 4.2 Windows 实机清单

- **CAP-T1** 连续截图 5000 次（全屏、区域各一半）：每 500 次记录 `GetGuiResources(GR_GDIOBJECTS / GR_USEROBJECTS)`，要求不增长；没有清理错误。修复前也跑一遍作对照。
- **CAP-T2** 区域截图与全屏截图对应位置逐像素一致，包括负坐标区域和部分越界区域。
- **CAP-T3** 主屏 150%、副屏 100% 且在左侧（负坐标）：全屏截图尺寸等于各屏物理分辨率的外接矩形，副屏内容不缩放、不错位。
- **CAP-T4** 分别在 Session 0（以服务运行）、锁屏、UAC 安全桌面、最小化的 RDP 会话下截图，记录是返回错误还是黑图、错误码是什么，结果写进文档。
- **CAP-T5** 进程是 PMv2，当前线程被设为 system-aware：返回线程文本的错误。线程恢复后截图成功。默认进程（未开启感知）返回原有文本的错误。
- **CAP-T6** 记录屏幕源 BitBlt 得到的原始 alpha 值，只作留档。
- **CAP-T7** 分层或半透明窗口（工具提示、OSD）：对比 SRCCOPY 与加 CAPTUREBLT 的结果。

## 5. 文档更新清单

- `API.md` 与 `API_CN.md` 的 Capture 小节（`### func CaptureVirtualDesktop` / `### func CaptureRegion`）：
  - 删去“内部调用 CaptureVirtualDesktop 后裁剪”的说法，改为只截交集。
  - 写明区域图的 (0,0) 是交集左上角，这样的图不能交给 `ImageToVirtual`（它只适用于整张虚拟桌面）。
  - 补 `CaptureRegionWithOptions` 和 `CaptureOptions` 各字段的说明（峰值上限、默认 1000、PreserveAlpha 已弃用）。
  - 补不支持的环境和黑图说明；写明 DPI 要求针对调用线程；写明“截图成功、清理失败”时会同时返回图像和错误。
- `screen/capture.go` 的 GoDoc：与上面一致；写明一次截图锁定一个 OS 线程，以及 GdiFlush 报告的是整个批次。
- `screen/monitor.go` 中 `ImageToVirtual` 的注释：补一句，区域截图不适用。
- `window/dpi.go` 的注释：更正函数名和最低版本（1803）。
- `README.md` 与 `README_CN.md` 的截图示例（同一段）：现在的示例把“或者截区域”和后面的 `ImageToVirtual` 写在一起，用区域图会换算错位。改成注明区域图不能用 `ImageToVirtual`（或者把区域示例改为自行加上交集原点）。如果补 Options 示例，两处一起补。
- `ClientInfo` 与 CaptureRegion 的坐标对齐条件不变：调用线程的 DPI 感知必须与截图一致。
- 若开发本地另有 `DESIGN_FORK_ABSORB.md`（它不在仓库及历史中，`docs/adr` 已被 `.gitignore` 忽略），把其中 “CaptureRegion 使用整张虚拟桌面的图像” 改为只截交集。
- 变更说明：MaxMemoryMB 改为峰值语义，默认值改为 1000，显式传值者可截像素数减半；PreserveAlpha 弃用，A 恒为 255；“截图成功、清理失败”时同时返回图像和错误。

## 6. 修订记录

- r3（实现时的实测，写回 CAP-05）：在本机上，线程被 `SetThreadDpiAwarenessContext` 设为 system-aware 之后，`GetDpiAwarenessContextForProcess(NULL)`、`GetDpiAwarenessContextForProcess(GetCurrentProcess())` 和 `GetProcessDpiAwareness` 都返回该线程的上下文（实测句柄 `0x6011`，awareness 为 1），不再是进程默认值。另一个 OS 线程上的 `IsPerMonitorDPIAware()` 仍为 true。因此“进程满足、线程不满足”的错误文本不能在当前线程上调用 `IsPerMonitorDPIAware` 来区分，截图改为到另一个未切换的 OS 线程上读取进程默认值。`Call(0)` 保持不变。
- r2（基线 `dd234a5`）：
  - 基线改为 `dd234a5`，行号改为“约”，以内容定位；proc.go 行号 +1；capture_screen 位置更新为 :387。
  - CAP-01：锁覆盖整个函数，Unlock 的 defer 最先注册；删除“先释放屏幕 DC 再转换”的写法；加锁后先清理残留批次；DeleteDC 纳入清理检查；失败路径也必须释放完；返回规则只保留一种（errors.Join）；BitBlt 失败时不调用 GdiFlush。
  - CAP-02：虚拟屏只在锁内读一次，不用 VirtualBounds；参数校验放在加锁之前；源坐标保留负值；固定 DIB 布局；写明 (0,0) 的含义。
  - CAP-03：拒绝乘法溢出；错误文本 MB 用峰值；修改 CaptureWithOptions 测试；记录“方案 A 已定”。
  - CAP-04：取消用桩函数替换 BitBlt，改为测纯函数；规定错误文本带数字码并用 %w 包装。
  - CAP-05：两个线程函数都存在时不再回退；按“进程是否满足”给出两种错误文本，都保留子串；补 DPI_AWARENESS 依据。
  - CAP-06：记录已定；CAP-07 改为先测后定；CAP-08 转换在锁内执行。
  - 文档清单：补 ImageToVirtual 限制、README/README_CN 示例的误导、DESIGN_FORK_ABSORB.md 条件修改、清理错误的返回语义。
