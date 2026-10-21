//go:build windows

package tui

import (
	"syscall"
	"unsafe"
)

// Флаги консоли Windows.
const (
	enableProcessedInput            = 0x0001
	enableLineInput                 = 0x0002
	enableEchoInput                 = 0x0004
	enableVirtualTerminalInput      = 0x0200
	enableProcessedOutput           = 0x0001
	enableVirtualTerminalProcessing = 0x0004
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procGetConsoleOutputCP         = kernel32.NewProc("GetConsoleOutputCP")
	procSetConsoleOutputCP         = kernel32.NewProc("SetConsoleOutputCP")
)

// rawState хранит прежние режимы консоли, чтобы вернуть их при выходе.
type rawState struct {
	inMode    uint32
	outMode   uint32
	outCP     uint32
	cpChanged bool
}

func consoleMode(handle syscall.Handle) (uint32, error) {
	var mode uint32
	ok, _, err := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return 0, err
	}
	return mode, nil
}

func setConsoleMode(handle syscall.Handle, mode uint32) error {
	ok, _, err := procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	if ok == 0 {
		return err
	}
	return nil
}

// enterRaw включает виртуальный терминал (VT) и отключает построчный ввод,
// чтобы TUI работал в conhost и Windows Terminal.
func enterRaw(fd int) (*rawState, error) {
	inMode, err := consoleMode(syscall.Stdin)
	if err != nil {
		return nil, err
	}
	outMode, err := consoleMode(syscall.Stdout)
	if err != nil {
		return nil, err
	}

	newIn := inMode &^ uint32(enableLineInput|enableEchoInput|enableProcessedInput)
	newIn |= enableVirtualTerminalInput
	newOut := outMode | enableProcessedOutput | enableVirtualTerminalProcessing

	state := &rawState{inMode: inMode, outMode: outMode}
	if err := setConsoleMode(syscall.Stdin, newIn); err != nil {
		return nil, err
	}
	if err := setConsoleMode(syscall.Stdout, newOut); err != nil {
		_ = setConsoleMode(syscall.Stdin, inMode)
		return nil, err
	}

	// Переключаем вывод в UTF-8, иначе кириллица превращается в мусор.
	if oldCP, _, _ := procGetConsoleOutputCP.Call(); oldCP != 0 {
		state.outCP = uint32(oldCP)
		if oldCP != 65001 {
			if ok, _, _ := procSetConsoleOutputCP.Call(65001); ok != 0 {
				state.cpChanged = true
			}
		}
	}
	return state, nil
}

func (s *rawState) restore() {
	if s == nil {
		return
	}
	_ = setConsoleMode(syscall.Stdin, s.inMode)
	_ = setConsoleMode(syscall.Stdout, s.outMode)
	if s.cpChanged {
		_, _, _ = procSetConsoleOutputCP.Call(uintptr(s.outCP))
	}
}

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

// termSize возвращает размер видимого окна консоли.
func termSize(fd int) (int, int) {
	var info consoleScreenBufferInfo
	ok, _, _ := procGetConsoleScreenBufferInfo.Call(
		uintptr(syscall.Stdout), uintptr(unsafe.Pointer(&info)),
	)
	if ok == 0 {
		return 80, 24
	}
	width := int(info.window.right-info.window.left) + 1
	height := int(info.window.bottom-info.window.top) + 1
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}
