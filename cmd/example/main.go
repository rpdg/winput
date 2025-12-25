package main

import (
	"log"
	"time"

	"github.com/rpdg/winput"
)

func main() {
	log.SetFlags(log.Ltime)
	log.Println("=== winput Library Example ===")

	winput.SetBackend(winput.BackendHID)

	// 1. 设置 DPI 感知 (推荐)
	if err := winput.EnablePerMonitorDPI(); err != nil {
		log.Printf("Warning: DPI awareness error: %v", err)
	}

	// 2. 寻找记事本窗口
	// 优先匹配标题，失败则匹配类名
	w, err := winput.FindByTitle("无标题 - 记事本")
	if err != nil {
		w, err = winput.FindByClass("Notepad")
	}

	if err != nil {
		log.Fatalf("❌ 未找到记事本窗口，请先打开记事本运行此示例。")
	}

	log.Printf("✅ 已连接窗口: %v", w.HWND)

	// 3. 鼠标交互
	// 坐标均为相对于窗口客户区的逻辑坐标
	log.Println("👉 正在执行鼠标操作...")
	w.Click(100, 100) // 左键点击
	time.Sleep(500 * time.Millisecond)

	// w.ClickRight(100, 100)  // 右键点击演示
	// w.Scroll(100, 100, 120) // 向上滚动演示

	// 4. 键盘交互
	log.Println("⌨️  正在执行键盘操作...")

	// 测试大写字母和符号 (Type 现在会自动处理 Shift)
	msg := "Hello WINPUT! 123 @#$"
	log.Printf("   - 正在输入: '%s'", msg)
	if err := w.Type(msg); err != nil {
		log.Printf("Type failed: %v", err)
	}

	// 按下回车
	w.Press(winput.KeyEnter)

	// 演示手动组合键 (例如 Ctrl + A 全选)
	log.Println("   - 执行组合键: Ctrl + A")
	w.KeyDown(winput.KeyCtrl)
	w.Press(winput.KeyA)
	w.KeyUp(winput.KeyCtrl)

	log.Println("✅ 演示完成。")
}
