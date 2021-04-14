package mopeka_pro_check

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
)

type Scanner struct {
	dur     *time.Duration
	stop    bool
	mutex   sync.RWMutex
	devices map[string]MopekaProCheck
}

func onStateChanged(device gatt.Device, s gatt.State) {
	switch s {
	case gatt.StatePoweredOn:
		fmt.Println("Scanning for Broadcasts...")
		device.Scan([]gatt.UUID{}, true)
		return
	default:
		device.StopScanning()
	}
}

func NewScanner(timeout time.Duration) Scanner {
	d, err := gatt.NewDevice(option.DefaultClientOptions...)
	if err != nil {
		log.Fatal("Can't create new device:", err)
	}
	scanner := Scanner{
		dur:     &timeout,
		mutex:   sync.RWMutex{},
		devices: make(map[string]MopekaProCheck),
	}
	d.Handle(gatt.PeripheralDiscovered(scanner.adScanHandler))
	d.Init(onStateChanged)
	return scanner
}

// Handle the advertisement scan
func (s *Scanner) adScanHandler(p gatt.Peripheral, a *gatt.Advertisement, rssi int) {
	if device, ok := ParseDevice(a, rssi); ok {
		s.mutex.Lock()
		s.devices[device.GetAddress()] = device
		s.mutex.Unlock()
	}
}

func (s *Scanner) adScanFilter(a gatt.Advertisement) bool {
	return FilterDevice(a)
}

func (s *Scanner) GetDevices() []MopekaProCheck {
	deviceList := make([]MopekaProCheck, 0, len(s.devices))
	for _, device := range s.devices {
		deviceList = append(deviceList, device)
	}
	return deviceList
}

func (s *Scanner) GetDevice(addr string) (MopekaProCheck, bool) {
	s.mutex.RLock()
	device, ok := s.devices[addr]
	s.mutex.RUnlock()
	return device, ok
}

// reformat string for proper display of hex
func formatHex(instr string) (outstr string) {
	outstr = ""
	for i := range instr {
		if i%2 == 0 {
			outstr += instr[i:i+2] + " "
		}
	}
	return
}

// clean up the non-ASCII characters
func clean(input string) string {
	return strings.TrimFunc(input, func(r rune) bool {
		return !unicode.IsGraphic(r)
	})
}
