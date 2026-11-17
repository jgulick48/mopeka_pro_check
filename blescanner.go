package mopeka_pro_check

import (
	"log"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Scanner collects the latest advertisement of every Mopeka sensor in range.
type Scanner struct {
	dev        int
	mutex      sync.RWMutex
	devices    map[string]MopekaProCheck
	lastUpdate time.Time
	stop       chan struct{}
	done       chan struct{}
	err        error
	seen       uint64
}

// NewScanner returns a scanner for Bluetooth adapter hci0. The timeout is kept
// for compatibility and is no longer used: scanning is continuous.
func NewScanner(_ time.Duration) *Scanner {
	return NewScannerForDevice(0)
}

// NewScannerForDevice returns a scanner for adapter hci<dev>.
func NewScannerForDevice(dev int) *Scanner {
	return &Scanner{dev: dev, devices: make(map[string]MopekaProCheck)}
}

func (s *Scanner) handle(address, name string, rssi int, mfr []byte) {
	device, ok := NewFromManufacturerData(address, name, rssi, mfr)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.seen++
	if !ok {
		return
	}
	s.devices[device.GetAddress()] = device
	s.lastUpdate = device.detected
}

// AdvertisementsSeen counts every advertisement with manufacturer data the
// adapter reported, Mopeka or not; a growing count shows the radio works.
func (s *Scanner) AdvertisementsSeen() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seen
}

func (s *Scanner) GetDevices() []MopekaProCheck {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	deviceList := make([]MopekaProCheck, 0, len(s.devices))
	for _, device := range s.devices {
		deviceList = append(deviceList, device)
	}
	return deviceList
}

func (s *Scanner) GetDevice(addr string) (MopekaProCheck, bool) {
	s.mutex.RLock()
	device, ok := s.devices[strings.ToUpper(addr)]
	s.mutex.RUnlock()
	return device, ok
}

// StartScan starts scanning in the background. Use Err to see why it stopped.
func (s *Scanner) StartScan() {
	s.mutex.Lock()
	if s.stop != nil {
		s.mutex.Unlock()
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.err = nil
	stop, done := s.stop, s.done
	s.mutex.Unlock()
	go func() {
		defer close(done)
		log.Printf("Started scanning on hci%d", s.dev)
		err := scanHCI(s.dev, stop, s.handle)
		s.mutex.Lock()
		s.err = err
		s.mutex.Unlock()
		if err != nil {
			log.Printf("Scanning on hci%d stopped: %v", s.dev, err)
		} else {
			log.Println("Stopped scanning.")
		}
	}()
}

// StopScan stops scanning and waits for the adapter to be released.
func (s *Scanner) StopScan() {
	s.mutex.Lock()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	s.mutex.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

// Running reports whether the scan loop is active.
func (s *Scanner) Running() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.done == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Err is the error that ended the last scan, if any.
func (s *Scanner) Err() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.err
}

func (s *Scanner) GetLastUpdateTime() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastUpdate
}

// clean up the non-printable characters
func clean(input string) string {
	return strings.TrimFunc(input, func(r rune) bool {
		return !unicode.IsGraphic(r)
	})
}
