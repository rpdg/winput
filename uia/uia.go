package uia

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	oleaut32 = syscall.NewLazyDLL("oleaut32.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procSysFreeString    = oleaut32.NewProc("SysFreeString")
	procSysStringLen     = oleaut32.NewProc("SysStringLen")
)

const (
	clsctxInprocServer   = 0x1
	coinitMultithreaded  = 0x0
	rpcEChangedMode      = 0x80010106
	treeScopeDescendants = 0x4

	uiaControlTypePropertyID = 30003
	uiaEditControlTypeID     = 50004
	uiaDocumentControlTypeID = 50030

	uiaValuePatternID = 10002
	uiaTextPatternID  = 10014

	vtI4 = 3
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidCUIAutomation = guid{0xFF48DBA4, 0x60EF, 0x4201, [8]byte{0xAA, 0x87, 0x54, 0x10, 0x3E, 0xEF, 0x59, 0x4E}}
	iidIUIAutomation   = guid{0x30CBE57D, 0xD9D0, 0x452A, [8]byte{0xAB, 0x13, 0x7A, 0xC5, 0xAC, 0x48, 0x25, 0xEE}}
)

type variant struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Val       int64
}

type iuiAutomation struct {
	lpVtbl *iuiAutomationVtbl
}

type iuiAutomationVtbl struct {
	QueryInterface              uintptr
	AddRef                      uintptr
	Release                     uintptr
	CompareElements             uintptr
	CompareRuntimeIDs           uintptr
	GetRootElement              uintptr
	ElementFromHandle           uintptr
	ElementFromPoint            uintptr
	GetFocusedElement           uintptr
	GetRootElementBuildCache    uintptr
	ElementFromHandleBuildCache uintptr
	ElementFromPointBuildCache  uintptr
	GetFocusedElementBuildCache uintptr
	CreateTreeWalker            uintptr
	GetControlViewWalker        uintptr
	GetContentViewWalker        uintptr
	GetRawViewWalker            uintptr
	GetRawViewCondition         uintptr
	GetControlViewCondition     uintptr
	GetContentViewCondition     uintptr
	CreateCacheRequest          uintptr
	CreateTrueCondition         uintptr
	CreateFalseCondition        uintptr
	CreatePropertyCondition     uintptr
}

type iuiAutomationCondition struct {
	lpVtbl *iunknownVtbl
}

type iuiAutomationElement struct {
	lpVtbl *iuiAutomationElementVtbl
}

type iuiAutomationElementVtbl struct {
	QueryInterface            uintptr
	AddRef                    uintptr
	Release                   uintptr
	SetFocus                  uintptr
	GetRuntimeID              uintptr
	FindFirst                 uintptr
	FindAll                   uintptr
	FindFirstBuildCache       uintptr
	FindAllBuildCache         uintptr
	BuildUpdatedCache         uintptr
	GetCurrentPropertyValue   uintptr
	GetCurrentPropertyValueEx uintptr
	GetCachedPropertyValue    uintptr
	GetCachedPropertyValueEx  uintptr
	GetCurrentPatternAs       uintptr
	GetCachedPatternAs        uintptr
	GetCurrentPattern         uintptr
	GetCachedPattern          uintptr
	GetCachedParent           uintptr
	GetCachedChildren         uintptr
	GetCurrentProcessID       uintptr
	GetCurrentControlType     uintptr
}

type iuiAutomationValuePattern struct {
	lpVtbl *iuiAutomationValuePatternVtbl
}

type iuiAutomationValuePatternVtbl struct {
	QueryInterface       uintptr
	AddRef               uintptr
	Release              uintptr
	SetValue             uintptr
	GetCurrentValue      uintptr
	GetCurrentIsReadOnly uintptr
	GetCachedValue       uintptr
	GetCachedIsReadOnly  uintptr
}

type iuiAutomationTextPattern struct {
	lpVtbl *iuiAutomationTextPatternVtbl
}

type iuiAutomationTextPatternVtbl struct {
	QueryInterface            uintptr
	AddRef                    uintptr
	Release                   uintptr
	RangeFromPoint            uintptr
	RangeFromChild            uintptr
	GetSelection              uintptr
	GetVisibleRanges          uintptr
	GetDocumentRange          uintptr
	GetSupportedTextSelection uintptr
}

type iuiAutomationTextRange struct {
	lpVtbl *iuiAutomationTextRangeVtbl
}

type iuiAutomationTextRangeVtbl struct {
	QueryInterface        uintptr
	AddRef                uintptr
	Release               uintptr
	Clone                 uintptr
	Compare               uintptr
	CompareEndpoints      uintptr
	ExpandToEnclosingUnit uintptr
	FindAttribute         uintptr
	FindText              uintptr
	GetAttributeValue     uintptr
	GetBoundingRectangles uintptr
	GetEnclosingElement   uintptr
	GetText               uintptr
}

type iunknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

func succeeded(hr uintptr) bool {
	return int32(hr) >= 0
}

func hresultErr(op string, hr uintptr) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, uint32(hr))
}

func coInitialize() (func(), error) {
	runtime.LockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	switch uint32(hr) {
	case 0, 1:
		return func() {
			procCoUninitialize.Call()
			runtime.UnlockOSThread()
		}, nil
	case rpcEChangedMode:
		return runtime.UnlockOSThread, nil
	default:
		runtime.UnlockOSThread()
		return nil, hresultErr("CoInitializeEx", hr)
	}
}

func releaseIUnknown(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	unk := (*iuiAutomationCondition)(ptr)
	syscall.SyscallN(unk.lpVtbl.Release, uintptr(ptr))
}

func createAutomation() (*iuiAutomation, error) {
	var automation *iuiAutomation
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIUIAutomation)),
		uintptr(unsafe.Pointer(&automation)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("CoCreateInstance(CUIAutomation)", hr)
	}
	return automation, nil
}

func (a *iuiAutomation) release() {
	if a == nil {
		return
	}
	syscall.SyscallN(a.lpVtbl.Release, uintptr(unsafe.Pointer(a)))
}

func (a *iuiAutomation) elementFromHandle(hwnd uintptr) (*iuiAutomationElement, error) {
	var elem *iuiAutomationElement
	hr, _, _ := syscall.SyscallN(
		a.lpVtbl.ElementFromHandle,
		uintptr(unsafe.Pointer(a)),
		hwnd,
		uintptr(unsafe.Pointer(&elem)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("IUIAutomation.ElementFromHandle", hr)
	}
	if elem == nil {
		return nil, fmt.Errorf("IUIAutomation.ElementFromHandle returned nil")
	}
	return elem, nil
}

func (a *iuiAutomation) createControlTypeCondition(controlType int32) (*iuiAutomationCondition, error) {
	var cond *iuiAutomationCondition
	value := variant{VT: vtI4, Val: int64(controlType)}
	hr, _, _ := syscall.SyscallN(
		a.lpVtbl.CreatePropertyCondition,
		uintptr(unsafe.Pointer(a)),
		uiaControlTypePropertyID,
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&cond)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("IUIAutomation.CreatePropertyCondition", hr)
	}
	if cond == nil {
		return nil, fmt.Errorf("IUIAutomation.CreatePropertyCondition returned nil")
	}
	return cond, nil
}

func (e *iuiAutomationElement) release() {
	if e == nil {
		return
	}
	syscall.SyscallN(e.lpVtbl.Release, uintptr(unsafe.Pointer(e)))
}

func (e *iuiAutomationElement) currentControlType() (int32, error) {
	var controlType int32
	hr, _, _ := syscall.SyscallN(
		e.lpVtbl.GetCurrentControlType,
		uintptr(unsafe.Pointer(e)),
		uintptr(unsafe.Pointer(&controlType)),
	)
	if !succeeded(hr) {
		return 0, hresultErr("IUIAutomationElement.get_CurrentControlType", hr)
	}
	return controlType, nil
}

func (e *iuiAutomationElement) findFirst(scope int32, cond *iuiAutomationCondition) (*iuiAutomationElement, error) {
	var found *iuiAutomationElement
	hr, _, _ := syscall.SyscallN(
		e.lpVtbl.FindFirst,
		uintptr(unsafe.Pointer(e)),
		uintptr(scope),
		uintptr(unsafe.Pointer(cond)),
		uintptr(unsafe.Pointer(&found)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("IUIAutomationElement.FindFirst", hr)
	}
	return found, nil
}

func (e *iuiAutomationElement) currentPattern(patternID int32) (unsafe.Pointer, error) {
	var pattern unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		e.lpVtbl.GetCurrentPattern,
		uintptr(unsafe.Pointer(e)),
		uintptr(patternID),
		uintptr(unsafe.Pointer(&pattern)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("IUIAutomationElement.GetCurrentPattern", hr)
	}
	if pattern == nil {
		return nil, nil
	}
	return pattern, nil
}

func (p *iuiAutomationValuePattern) release() {
	if p == nil {
		return
	}
	syscall.SyscallN(p.lpVtbl.Release, uintptr(unsafe.Pointer(p)))
}

func (p *iuiAutomationValuePattern) currentValue() (string, error) {
	var bstr uintptr
	hr, _, _ := syscall.SyscallN(
		p.lpVtbl.GetCurrentValue,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&bstr)),
	)
	if !succeeded(hr) {
		return "", hresultErr("IUIAutomationValuePattern.get_CurrentValue", hr)
	}
	return bstrToStringAndFree(bstr), nil
}

func (p *iuiAutomationTextPattern) release() {
	if p == nil {
		return
	}
	syscall.SyscallN(p.lpVtbl.Release, uintptr(unsafe.Pointer(p)))
}

func (p *iuiAutomationTextPattern) documentRange() (*iuiAutomationTextRange, error) {
	var textRange *iuiAutomationTextRange
	hr, _, _ := syscall.SyscallN(
		p.lpVtbl.GetDocumentRange,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&textRange)),
	)
	if !succeeded(hr) {
		return nil, hresultErr("IUIAutomationTextPattern.get_DocumentRange", hr)
	}
	if textRange == nil {
		return nil, fmt.Errorf("IUIAutomationTextPattern.get_DocumentRange returned nil")
	}
	return textRange, nil
}

func (r *iuiAutomationTextRange) release() {
	if r == nil {
		return
	}
	syscall.SyscallN(r.lpVtbl.Release, uintptr(unsafe.Pointer(r)))
}

func (r *iuiAutomationTextRange) text() (string, error) {
	var bstr uintptr
	hr, _, _ := syscall.SyscallN(
		r.lpVtbl.GetText,
		uintptr(unsafe.Pointer(r)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&bstr)),
	)
	if !succeeded(hr) {
		return "", hresultErr("IUIAutomationTextRange.GetText", hr)
	}
	return bstrToStringAndFree(bstr), nil
}

func bstrToStringAndFree(bstr uintptr) string {
	if bstr == 0 {
		return ""
	}
	defer procSysFreeString.Call(bstr)
	n, _, _ := procSysStringLen.Call(bstr)
	if n == 0 {
		return ""
	}
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(bstr)), int(n))
	return string(utf16.Decode(buf))
}

func findCandidateElement(automation *iuiAutomation, root *iuiAutomationElement) (*iuiAutomationElement, error) {
	controlType, err := root.currentControlType()
	if err == nil && (controlType == uiaEditControlTypeID || controlType == uiaDocumentControlTypeID) {
		return root, nil
	}

	for _, candidateType := range []int32{uiaEditControlTypeID, uiaDocumentControlTypeID} {
		cond, err := automation.createControlTypeCondition(candidateType)
		if err != nil {
			return nil, err
		}
		found, findErr := root.findFirst(treeScopeDescendants, cond)
		releaseIUnknown(unsafe.Pointer(cond))
		if findErr != nil {
			return nil, findErr
		}
		if found != nil {
			return found, nil
		}
	}

	return root, nil
}

func readElementValue(elem *iuiAutomationElement) (string, error) {
	pattern, err := elem.currentPattern(uiaValuePatternID)
	if err == nil && pattern != nil {
		valuePattern := (*iuiAutomationValuePattern)(pattern)
		defer valuePattern.release()
		return valuePattern.currentValue()
	}

	pattern, err = elem.currentPattern(uiaTextPatternID)
	if err == nil && pattern != nil {
		textPattern := (*iuiAutomationTextPattern)(pattern)
		defer textPattern.release()
		textRange, err := textPattern.documentRange()
		if err != nil {
			return "", err
		}
		defer textRange.release()
		return textRange.text()
	}

	return "", fmt.Errorf("no readable UI Automation pattern available")
}

// GetText reads text from a window/control using Windows UI Automation.
func GetText(hwnd uintptr) (string, error) {
	cleanup, err := coInitialize()
	if err != nil {
		return "", err
	}
	defer cleanup()

	automation, err := createAutomation()
	if err != nil {
		return "", err
	}
	defer automation.release()

	root, err := automation.elementFromHandle(hwnd)
	if err != nil {
		return "", err
	}
	defer root.release()

	target, err := findCandidateElement(automation, root)
	if err != nil {
		return "", err
	}
	if target != root {
		defer target.release()
	}

	return readElementValue(target)
}
