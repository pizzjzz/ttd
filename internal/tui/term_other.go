//go:build !linux && !darwin && !windows

package tui

import "errors"

type rawState struct{}

// enterRaw не поддерживается на этой платформе — TUI перейдёт в простой режим.
func enterRaw(fd int) (*rawState, error) {
	return nil, errors.New("raw-режим не поддерживается на этой платформе")
}

func (s *rawState) restore() {}

// termSize возвращает размер по умолчанию.
func termSize(fd int) (int, int) { return 80, 24 }
