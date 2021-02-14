package mopeka_pro_check

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/sausheong/ble"
	"github.com/sausheong/ble/linux"
)

type Scanner struct {
	dur     *time.Duration
	stop    bool
	mutex   sync.RWMutex
	devices map[string]MopekaProCheck
}

func NewScanner(timeout time.Duration) Scanner {
	d, err := linux.NewDevice()
	if err != nil {
		log.Fatal("Can't create new device:", err)
	}
	ble.SetDefaultDevice(d)

	return Scanner{
		dur:     &timeout,
		mutex:   sync.RWMutex{},
		devices: make(map[string]MopekaProCheck),
	}
}

// Handle the advertisement scan
func (s *Scanner) adScanHandler(a ble.Advertisement) {
	s.mutex.Lock()
	if device, ok := ParseDevice(a); ok {
		s.devices[device.GetAddress()] = device
	}
	s.mutex.Unlock()
}

func (s *Scanner) GetDevices() []MopekaProCheck {
	deviceList := make([]MopekaProCheck, 0, len(s.devices))
	for _, device := range s.devices {
		deviceList = append(deviceList, device)
	}
	return deviceList
}

func (s *Scanner) GetDevice(addr string) (MopekaProCheck, bool) {
	s.mutex.Lock()
	device, ok := s.devices[addr]
	s.mutex.Unlock()
	return device, ok
}

// handler to start scanning
func (s *Scanner) StartScan() {
	go s.scan()
}

// handler to stop scanning
func (s *Scanner) StopScan() {
	s.stop = true
}

// scan goroutine
func (s *Scanner) scan() {
	s.stop = false
	log.Println("Started scanning every", *s.dur)
	for !s.stop {
		ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), *s.dur))
		err := ble.Scan(ctx, false, s.adScanHandler, nil)
		if err != nil {
			log.Printf("Error scanning for devices %s", err)
		}
	}
	log.Println("Stopped scanning.")
	s.stop = true
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
