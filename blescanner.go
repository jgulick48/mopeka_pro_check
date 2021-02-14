package mopeka_pro_check

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/sausheong/ble"
)

var dur *time.Duration
var logger *log.Logger
var stop bool = true
var mutex sync.RWMutex
var devices map[string]MopekaProCheck

// Handle the advertisement scan
func adScanHandler(a ble.Advertisement) {
	mutex.Lock()
	if device, ok := ParseDevice(a); ok {
		devices[device.GetAddress()] = device
	}
	mutex.Unlock()
}

func GetDevices() []MopekaProCheck {
	deviceList := make([]MopekaProCheck, 0, len(devices))
	for _, device := range devices {
		deviceList = append(deviceList, device)
	}
	return deviceList
}

// handler to start scanning
func StartScan() {
	go scan()
}

// handler to stop scanning
func StopScan() {
	stop = true
}

// scan goroutine
func scan() {
	stop = false
	logger.Println("Started scanning every", *dur)
	for !stop {
		ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), *dur))
		ble.Scan(ctx, false, adScanHandler, nil)
	}
	logger.Println("Stopped scanning.")
	stop = true
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
