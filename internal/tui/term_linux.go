//go:build linux

package tui

import (
	"syscall"
	"unsafe"
)

type rawState struct {
	fd  int
	old syscall.Termios
}

// enterRaw переводит терминал в необработанный режим (raw mode).
func enterRaw(fd int) (*rawState, error) {
	var old syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&old)), 0, 0, 0,
	); errno != 0 {
		return nil, errno
	}
	raw := old
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.BRKINT | syscall.INPCK | syscall.ISTRIP
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&raw)), 0, 0, 0,
	); errno != 0 {
		return nil, errno
	}
	return &rawState{fd: fd, old: old}, nil
}

func (s *rawState) restore() {
	if s == nil {
		return
	}
	syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(s.fd), uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&s.old)), 0, 0, 0,
	)
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// termSize возвращает ширину и высоту терминала.
func termSize(fd int) (int, int) {
	var ws winsize
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)), 0, 0, 0,
	); errno != 0 {
		return 80, 24
	}
	width, height := int(ws.Col), int(ws.Row)
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}
