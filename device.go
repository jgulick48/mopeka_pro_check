package mopeka_pro_check

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// MOPEKA_MANUFACTURER_ID is the Bluetooth SIG company ID in Mopeka Pro
// advertisements (Nordic Semiconductor).
const MOPEKA_MANUFACTURER_ID = 0x0059

var MopekaTankLevelCoefficientsPropane = []float64{0.573045, -0.002822, -0.00000535}

// TankTypes maps a tank type to its height in mm: the inside height for
// vertical tanks ("_v") and the inside diameter for horizontal tanks ("_h").
var TankTypes = map[string]float64{
	"20lb_v":  302.84,
	"30lb_v":  400,
	"40lb_v":  498.62,
	"100lb_v": 1300,
	"500g_h":  939.8,
}

// TankVolumeGallons is the water capacity of the tank types that are sold by
// gallons. Callers can use it to turn a fill percentage into gallons.
var TankVolumeGallons = map[string]float64{
	"500g_h": 500,
}

// SensorTypes maps the hardware ID (third byte of the manufacturer data).
var SensorTypes = map[byte]string{0x3: "Standard Propane", 0x4: "Top down air space", 0x5: "Bottom up water"}

// MopekaProCheck is one decoded advertisement from a Mopeka Pro sensor.
type MopekaProCheck struct {
	address  string
	detected time.Time
	name     string
	rssi     int
	data     []byte
}

// NewFromManufacturerData decodes Mopeka manufacturer data. data includes the
// two company ID bytes (59 00) and is 12 bytes long.
func NewFromManufacturerData(address, name string, rssi int, data []byte) (MopekaProCheck, bool) {
	if len(data) != 12 || data[0] != byte(MOPEKA_MANUFACTURER_ID&0xFF) || data[1] != byte(MOPEKA_MANUFACTURER_ID>>8) {
		return MopekaProCheck{}, false
	}
	d := make([]byte, len(data))
	copy(d, data)
	return MopekaProCheck{
		address:  strings.ToUpper(address),
		detected: time.Now(),
		name:     clean(name),
		rssi:     rssi,
		data:     d,
	}, true
}

func (d *MopekaProCheck) getRawTemp() float64 {
	return float64(d.data[4] & 0x7F)
}

func (d *MopekaProCheck) GetReadingTime() time.Time {
	return d.detected
}

func (d *MopekaProCheck) GetName() string {
	return d.name
}

func (d *MopekaProCheck) GetTempCelsius() float64 {
	return d.getRawTemp() - 40
}

func (d *MopekaProCheck) GetTempFahrenheit() float64 {
	return (d.GetTempCelsius() * 1.8) + 32
}

// GetTankLevelMM is the liquid height above the sensor. Mopeka's propane
// coefficients take the raw temperature byte (°C + 40).
func (d *MopekaProCheck) GetTankLevelMM() float64 {
	a := int(d.data[6]) << 8
	b := int(d.data[5])
	rawTankLevel := (a + b) & 0x3FFF
	return float64(rawTankLevel) * (MopekaTankLevelCoefficientsPropane[0] + (MopekaTankLevelCoefficientsPropane[1] * d.getRawTemp()) + (MopekaTankLevelCoefficientsPropane[2] * d.getRawTemp() * d.getRawTemp()))
}

func (d *MopekaProCheck) GetTankLevelInches() float64 {
	return d.GetTankLevelMM() / 25.4
}

// GetLevelPercent is the fill percentage by volume. Horizontal tanks ("_h") are
// treated as a cylinder lying on its side; vertical tanks are linear.
func (d *MopekaProCheck) GetLevelPercent(tankType string) float64 {
	height, ok := TankTypes[tankType]
	if !ok {
		return 0
	}
	level := d.GetTankLevelMM()
	if level >= height {
		return 100
	}
	if strings.HasSuffix(tankType, "_h") {
		return CalculatePercentOfCircle(level, height/2)
	}
	return (level / height) * 100
}

// GetReadQuality is 0 (worst) to 3 (best).
func (d *MopekaProCheck) GetReadQuality() float64 {
	return float64(d.data[6] >> 6)
}

func (d *MopekaProCheck) GetXAccel() byte {
	return d.data[10]
}

func (d *MopekaProCheck) GetYAccel() byte {
	return d.data[11]
}

func (d *MopekaProCheck) GetRSSI() int {
	return d.rssi
}

func (d *MopekaProCheck) GetAddress() string {
	return d.address
}

// GetHardwareID is the raw hardware ID byte.
func (d *MopekaProCheck) GetHardwareID() byte {
	return d.data[2]
}

// GetSensorType is a name for known hardware IDs, otherwise "unknown (0xNN)".
func (d *MopekaProCheck) GetSensorType() string {
	if t, ok := SensorTypes[d.data[2]]; ok {
		return t
	}
	return fmt.Sprintf("unknown (0x%02x)", d.data[2])
}

// GetSyncPressed reports whether the sensor's sync button is held.
func (d *MopekaProCheck) GetSyncPressed() bool {
	return d.data[4]&0x80 != 0
}

func (d *MopekaProCheck) GetBatteryLevel() int {
	batteryVoltage := d.GetBatteryVoltage()
	percent := ((batteryVoltage - 2.2) / 0.65) * 100
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return int(math.Round(percent))
}

func (d *MopekaProCheck) GetBatteryVoltage() float64 {
	return float64(d.data[3]&0x7F) / 32
}

// CalculatePercentOfCircle is the filled share (percent) of a circle of radius
// r filled to height, i.e. the fill of a horizontal cylinder.
func CalculatePercentOfCircle(height float64, r float64) float64 {
	fullVol := math.Pi * math.Pow(r, 2)
	if height <= 0 {
		return 0
	}
	if height >= 2*r {
		return 100
	}
	if height == r {
		return .5 * 100
	}
	if height < r {
		theta := 2 * math.Acos((r-height)/r)
		vol := .5 * math.Pow(r, 2) * (theta - math.Sin(theta))
		return (vol / fullVol) * 100
	}
	theta := 2 * math.Acos((height-r)/r)
	vol := fullVol - .5*math.Pow(r, 2)*(theta-math.Sin(theta))
	return (vol / fullVol) * 100
}
