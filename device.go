package mopeka_pro_check

import (
	"math"
	"time"

	"github.com/sausheong/ble"
)

const MOPEKA_MANUFACTURER_ID = 0x0059

var MopekaTankLevelCoefficientsPropane = []float64{0.573045, -0.002822, -0.00000535}
var TankTypes = map[string]float64{
	"20lb_v": 302.84,
	"30lb_v": 400,
	"40lb_v": 498.62,
}
var SensorTypes = map[byte]string{0x3: "Standard Propane", 0x4: "Top down air space", 0x5: "Bottom up water"}

// MopekaProCheck represents a BLE device
type MopekaProCheck struct {
	address  string
	detected time.Time
	name     string
	rssi     int
	data     []byte
}

func (d *MopekaProCheck) getRawTemp() float64 {
	return float64(d.data[4] & 0x7F)
}

func (d *MopekaProCheck) GetTempCelsius() float64 {
	return d.getRawTemp() - 40
}
func (d *MopekaProCheck) GetTempFahrenheit() float64 {
	return (d.GetTempCelsius() * 1.8) + 32
}
func (d *MopekaProCheck) GetTankLevelMM() float64 {
	a := int(d.data[6]) << 8
	b := int(d.data[5])
	rawTankLevel := (a + b) & 0x3FFF
	return float64(rawTankLevel) * (MopekaTankLevelCoefficientsPropane[0] + (MopekaTankLevelCoefficientsPropane[1] * d.getRawTemp()) + (MopekaTankLevelCoefficientsPropane[2] * d.getRawTemp() * d.getRawTemp()))
}
func (d *MopekaProCheck) GetTankLevelInches() float64 {
	return d.GetTankLevelMM() - 25.4
}
func (d *MopekaProCheck) GetLevelPercent(tankType string) float64 {
	if height, ok := TankTypes[tankType]; ok {
		if d.GetTankLevelMM() < height {
			return d.GetTankLevelMM() / height
		} else {
			return 100
		}
	}
	return 0

}
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
func (d *MopekaProCheck) GetSensorType() string {
	return SensorTypes[d.data[2]]
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

func ParseDevice(a ble.Advertisement) (MopekaProCheck, bool) {
	data := a.ManufacturerData()
	if len(data) == 0 || data[0] != MOPEKA_MANUFACTURER_ID || len(data) != 12 {
		return MopekaProCheck{}, false
	}
	return MopekaProCheck{
		address:  a.Addr().String(),
		detected: time.Now(),
		name:     clean(a.LocalName()),
		rssi:     a.RSSI(),
		data:     data,
	}, true
}
