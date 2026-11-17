//go:build linux

package mopeka_pro_check

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func ioctlPtr(fd int, req uint, buf []byte) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return errno
	}
	return nil
}
