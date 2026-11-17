//go:build linux

package mopeka_pro_check

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Passive BLE scanning over a raw HCI socket. This needs CAP_NET_RAW and
// CAP_NET_ADMIN (root, or a privileged container with host networking) but no
// BlueZ: if bluetoothd is running it shares the adapter.

const (
	hciDevUp     = 0x400448C9
	hciDevDown   = 0x400448CA
	hciGetDevInf = 0x800448D3
	solHCI       = 0
	hciFilter    = 2
	hciEventPkt  = 0x04
	evtLEMeta    = 0x3E
	leAdvReport  = 0x02
)

func hciCommand(fd int, ogf, ocf uint16, params []byte) error {
	op := ogf<<10 | ocf
	pkt := []byte{0x01, byte(op), byte(op >> 8), byte(len(params))}
	_, err := unix.Write(fd, append(pkt, params...))
	time.Sleep(100 * time.Millisecond)
	return err
}

func devIsUp(ctl, dev int) (bool, error) {
	// struct hci_dev_info: dev_id u16, name[8], bdaddr[6], flags u32, ...
	buf := make([]byte, 92)
	binary.LittleEndian.PutUint16(buf, uint16(dev))
	if err := ioctlPtr(ctl, hciGetDevInf, buf); err != nil {
		return false, err
	}
	return binary.LittleEndian.Uint32(buf[16:])&1 == 1, nil
}

func scanHCI(dev int, stop <-chan struct{}, handle func(address, name string, rssi int, mfr []byte)) error {
	ctl, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.BTPROTO_HCI)
	if err != nil {
		return fmt.Errorf("bluetooth socket: %w", err)
	}
	defer unix.Close(ctl)
	wasUp, err := devIsUp(ctl, dev)
	if err != nil {
		return fmt.Errorf("hci%d info: %w", dev, err)
	}
	if !wasUp {
		if err := unix.IoctlSetInt(ctl, hciDevUp, dev); err != nil && !errors.Is(err, unix.EALREADY) {
			return fmt.Errorf("hci%d up: %w", dev, err)
		}
		defer unix.IoctlSetInt(ctl, hciDevDown, dev)
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.BTPROTO_HCI)
	if err != nil {
		return fmt.Errorf("hci socket: %w", err)
	}
	defer unix.Close(fd)
	if err := unix.Bind(fd, &unix.SockaddrHCI{Dev: uint16(dev), Channel: unix.HCI_CHANNEL_RAW}); err != nil {
		return fmt.Errorf("hci%d bind: %w", dev, err)
	}
	// struct hci_filter {type_mask u32; event_mask [2]u32; opcode u16} padded to 16
	filter := make([]byte, 16)
	binary.LittleEndian.PutUint32(filter[0:], 1<<hciEventPkt)
	binary.LittleEndian.PutUint32(filter[4:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(filter[8:], 0xFFFFFFFF)
	if err := unix.SetsockoptString(fd, solHCI, hciFilter, string(filter)); err != nil {
		return fmt.Errorf("hci filter: %w", err)
	}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		return fmt.Errorf("hci timeout: %w", err)
	}

	// Passive scan, interval = window = 100 ms (continuous), public address,
	// accept all, no duplicate filtering (every advertisement is reported).
	_ = hciCommand(fd, 0x08, 0x000C, []byte{0x00, 0x00})
	if err := hciCommand(fd, 0x08, 0x000B, []byte{0x00, 0xA0, 0x00, 0xA0, 0x00, 0x00, 0x00}); err != nil {
		return fmt.Errorf("le set scan parameters: %w", err)
	}
	if err := hciCommand(fd, 0x08, 0x000C, []byte{0x01, 0x00}); err != nil {
		return fmt.Errorf("le set scan enable: %w", err)
	}
	defer hciCommand(fd, 0x08, 0x000C, []byte{0x00, 0x00})

	buf := make([]byte, 1024)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		n, err := unix.Read(fd, buf)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("hci read: %w", err)
		}
		parseAdvertisingReports(buf[:n], handle)
	}
}

// parseAdvertisingReports decodes an HCI LE Advertising Report event and calls
// handle for every report that has manufacturer specific data.
func parseAdvertisingReports(pkt []byte, handle func(address, name string, rssi int, mfr []byte)) {
	if len(pkt) < 5 || pkt[0] != hciEventPkt || pkt[1] != evtLEMeta || pkt[3] != leAdvReport {
		return
	}
	count := int(pkt[4])
	off := 5
	for i := 0; i < count; i++ {
		if off+9 > len(pkt) {
			return
		}
		addr := pkt[off+2 : off+8]
		dlen := int(pkt[off+8])
		if off+9+dlen+1 > len(pkt) {
			return
		}
		data := pkt[off+9 : off+9+dlen]
		rssi := int(int8(pkt[off+9+dlen]))
		off += 10 + dlen

		var name string
		var mfr []byte
		for j := 0; j < len(data); {
			l := int(data[j])
			if l == 0 || j+1+l > len(data) {
				break
			}
			t, v := data[j+1], data[j+2:j+1+l]
			switch t {
			case 0x08, 0x09:
				name = string(v)
			case 0xFF:
				mfr = v
			}
			j += 1 + l
		}
		if mfr == nil {
			continue
		}
		parts := make([]string, 6)
		for k := 0; k < 6; k++ {
			parts[k] = fmt.Sprintf("%02X", addr[5-k])
		}
		handle(strings.Join(parts, ":"), name, rssi, mfr)
	}
}
