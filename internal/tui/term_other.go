//go:build !linux

package tui

import "errors"

type rawState struct{}

// enterRaw не поддерживается вне Linux.
func enterRaw(fd int) (*rawState, error) {
	return nil, errors.New("raw-режим не поддерживается на этой платформе")
}

func (s *rawState) restore() {}

// termSize возвращает размер по умолчанию.
func termSize(fd int) (int, int) { return 80, 24 }
