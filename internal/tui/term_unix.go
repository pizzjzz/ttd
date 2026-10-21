//go:build linux || darwin

package tui

import (
	"syscall"
	"unsafe"
)

// rawState хранит прежние настройки терминала, чтобы вернуть их при выходе.
type rawState struct {
	fd  int
	old syscall.Termios
}

func ioctl(fd int, request uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg), 0, 0, 0,
	); errno != 0 {
		return errno
	}
	return nil
}

// enterRaw переводит терминал в необработанный режим (raw mode).
func enterRaw(fd int) (*rawState, error) {
	var old syscall.Termios
	if err := ioctl(fd, termiosGet, unsafe.Pointer(&old)); err != nil {
		return nil, err
	}
	raw := old
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.BRKINT | syscall.INPCK | syscall.ISTRIP
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, termiosSet, unsafe.Pointer(&raw)); err != nil {
		return nil, err
	}
	return &rawState{fd: fd, old: old}, nil
}

func (s *rawState) restore() {
	if s == nil {
		return
	}
	_ = ioctl(s.fd, termiosSet, unsafe.Pointer(&s.old))
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
	if err := ioctl(fd, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err != nil {
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
