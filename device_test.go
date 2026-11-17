package mopeka_pro_check

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultBudgetAllocation(t *testing.T) {
	filled := CalculatePercentOfCircle(13, 18.5)
	assert.Equal(t, 31.356090324437847, filled)
}

func Test_DefaultBudgetAllocatio_MoreThanHalf(t *testing.T) {
	filled := CalculatePercentOfCircle(29, 18.5)
	assert.Equal(t, 84.08596089292737, filled)
}

// A real advertisement from a Mopeka Pro (hardware 0x08) on a 500 gal tank.
const sample = "5900086e3738c64bb69e50e2"

func Test_DecodeSample(t *testing.T) {
	data, _ := hex.DecodeString(sample)
	d, ok := NewFromManufacturerData("e4:c4:15:4b:b6:9e", "", -79, data)
	assert.True(t, ok)
	assert.Equal(t, "E4:C4:15:4B:B6:9E", d.GetAddress())
	assert.InDelta(t, 3.44, d.GetBatteryVoltage(), 0.01)
	assert.Equal(t, 15.0, d.GetTempCelsius())
	assert.Equal(t, 3.0, d.GetReadQuality())
	assert.InDelta(t, 639.4, d.GetTankLevelMM(), 0.5)
	assert.Equal(t, "unknown (0x08)", d.GetSensorType())
	assert.False(t, d.GetSyncPressed())
}

func Test_HorizontalTankUsesCylinder(t *testing.T) {
	data, _ := hex.DecodeString(sample)
	d, _ := NewFromManufacturerData("E4:C4:15:4B:B6:9E", "", -79, data)
	// 639 mm in a 939.8 mm diameter tank is ~72% by volume, not 68% linear.
	assert.InDelta(t, 72.2, d.GetLevelPercent("500g_h"), 0.5)
	assert.Equal(t, 0.0, d.GetLevelPercent("nope"))
}

func Test_RejectsOtherData(t *testing.T) {
	_, ok := NewFromManufacturerData("x", "", 0, []byte{0x59, 0x00, 1})
	assert.False(t, ok)
	data, _ := hex.DecodeString("4c00086e3738c64bb69e50e2")
	_, ok = NewFromManufacturerData("x", "", 0, data)
	assert.False(t, ok)
}

func Test_ParseAdvertisingReport(t *testing.T) {
	// HCI event: LE meta, adv report, 1 report, ADV_NONCONN_IND, public addr,
	// AD: flags + manufacturer data, rssi -79.
	ad, _ := hex.DecodeString("020106" + "0dff" + sample)
	pkt := []byte{0x04, 0x3E, 0, 0x02, 1, 0x03, 0x00, 0x9e, 0xb6, 0x4b, 0x15, 0xc4, 0xe4, byte(len(ad))}
	pkt = append(pkt, ad...)
	rssi := int8(-79)
	pkt = append(pkt, byte(rssi))
	pkt[2] = byte(len(pkt) - 3)
	var got []string
	parseAdvertisingReports(pkt, func(a, n string, r int, m []byte) {
		got = append(got, a)
		assert.Equal(t, -79, r)
		assert.Equal(t, sample, hex.EncodeToString(m))
	})
	assert.Equal(t, []string{"E4:C4:15:4B:B6:9E"}, got)
}
