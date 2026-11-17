//go:build !linux

package mopeka_pro_check

import "errors"

func scanHCI(int, <-chan struct{}, func(string, string, int, []byte)) error {
	return errors.New("BLE scanning is only supported on Linux")
}
