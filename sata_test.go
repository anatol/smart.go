package smart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseAsTemperatureRange(t *testing.T) {
	t.Parallel()
	raw := computeAttributeRawValue(ataDefaultAttributes[194], [6]byte{41, 0, 0, 0, 61, 0}, 0, 59, 39)

	a := AtaSmartAttr{Id: 194, Flags: 32, Current: 59, Worst: 39, VendorBytes: [6]byte{41, 0, 0, 0, 61, 0}, Name: "Temperature_Celsius", Type: AtaDeviceAttributeTypeTempMinMax, ValueRaw: raw}
	i1, i2, i3, i4, err := a.ParseAsTemperature()
	require.NoError(t, err)
	require.Equal(t, 41, i1)
	require.Equal(t, 0, i2)
	require.Equal(t, 61, i3)
	require.Equal(t, 0, i4)
}

func TestFromAtaString(t *testing.T) {
	t.Parallel()

	// ATA strings swap each pair of bytes; trailing spaces are trimmed.
	// "AB" is stored as [B, A], "ABCD" as [B, A, D, C], etc.
	require.Equal(t, "AB", fromAtaString([]byte{'B', 'A'}))
	require.Equal(t, "ABCD", fromAtaString([]byte{'B', 'A', 'D', 'C'}))
	// "SAMSUNG" encoded: each pair swapped → 'A','S','S','M','N','U',' ','G'
	require.Equal(t, "SAMSUNG", fromAtaString([]byte{'A', 'S', 'S', 'M', 'N', 'U', ' ', 'G'}))
	// All spaces → empty string after trim
	require.Equal(t, "", fromAtaString([]byte{' ', ' ', ' ', ' '}))
}

func TestComputeAttributeRawValue(t *testing.T) {
	t.Parallel()

	// Raw48: bytes 5..0 of vendorBytes packed MSB-first into 48 bits.
	mapping := ataDeviceAttr{typ: AtaDeviceAttributeTypeRaw48}
	raw := computeAttributeRawValue(mapping, [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, 0, 0, 0)
	require.Equal(t, uint64(0x060504030201), raw)

	// Raw64: vendorBytes + worst + current packed into 64 bits.
	mapping64 := ataDeviceAttr{typ: AtaDeviceAttributeTypeRaw64}
	raw64 := computeAttributeRawValue(mapping64, [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, 0, 0xAA /*current/v*/, 0xBB /*worst/w*/)
	// byteOrder "543210wv": 0x06,0x05,0x04,0x03,0x02,0x01,0xBB,0xAA
	require.Equal(t, uint64(0x0605040302_01_BB_AA), raw64)
}

func TestParseAsDuration(t *testing.T) {
	t.Parallel()

	attr := func(typ int, raw uint64) AtaSmartAttr {
		return AtaSmartAttr{Type: typ, ValueRaw: raw}
	}

	d, err := attr(AtaDeviceAttributeTypeSec2Hour, 7261).ParseAsDuration()
	require.NoError(t, err)
	require.Equal(t, 2*time.Hour+1*time.Minute+1*time.Second, d)

	d, err = attr(AtaDeviceAttributeTypeMin2Hour, 121).ParseAsDuration()
	require.NoError(t, err)
	require.Equal(t, 2*time.Hour+1*time.Minute, d)

	d, err = attr(AtaDeviceAttributeTypeHalfMin2Hour, 4).ParseAsDuration()
	require.NoError(t, err)
	require.Equal(t, 2*time.Minute, d)

	// Msec24Hour32: hours in lower 32 bits, milliseconds in upper 32 bits.
	rawMsec := (uint64(500) << 32) | uint64(3)
	d, err = attr(AtaDeviceAttributeTypeMsec24Hour32, rawMsec).ParseAsDuration()
	require.NoError(t, err)
	require.Equal(t, 3*time.Hour+500*time.Millisecond, d)

	_, err = attr(AtaDeviceAttributeTypeRaw48, 0).ParseAsDuration()
	require.Error(t, err)
}

func TestChecksum(t *testing.T) {
	t.Parallel()

	// A valid 512-byte block has all bytes summing to 0 mod 256.
	data := make([]byte, 512)
	// All zeros: sum is 0 → valid.
	require.True(t, checksum(data))

	data[0] = 0x01
	data[511] = 0xFF // 0x01 + 0xFF = 0x00 mod 256 → still valid.
	require.True(t, checksum(data))

	data[511] = 0x00 // now sum is 0x01 → invalid.
	require.False(t, checksum(data))
}

func TestCheckTempWord(t *testing.T) {
	t.Parallel()

	// 0..127: valid as both signed byte and signed word
	require.Equal(t, 0x11, checkTempWord(0))
	require.Equal(t, 0x11, checkTempWord(0x7f))
	// 128..255: valid as signed byte only (negative as byte)
	require.Equal(t, 0x01, checkTempWord(0x80))
	require.Equal(t, 0x01, checkTempWord(0xff))
	// 0xff80..0xffff: valid as signed word only (negative as word)
	require.Equal(t, 0x10, checkTempWord(0xff80))
	require.Equal(t, 0x10, checkTempWord(0xffff))
	// everything else: not a plausible temperature
	require.Equal(t, 0x00, checkTempWord(0x100))
	require.Equal(t, 0x00, checkTempWord(0x8000))
}

func TestCheckTempRange(t *testing.T) {
	t.Parallel()

	var lo, hi int8

	// Normal range: t inside [t1, t2]
	require.True(t, checkTempRange(20, 10, 30, &lo, &hi))
	require.Equal(t, int8(10), lo)
	require.Equal(t, int8(30), hi)

	// t1 and t2 are swapped: function normalizes them
	require.True(t, checkTempRange(20, 30, 10, &lo, &hi))
	require.Equal(t, int8(10), lo)
	require.Equal(t, int8(30), hi)

	// Degenerate case: lo==-1 and hi<=0 (uninitialized data)
	require.False(t, checkTempRange(0, -1, 0, &lo, &hi))

	// Temperature outside range
	require.False(t, checkTempRange(5, 10, 30, &lo, &hi))

	// Range exceeds bounds (-60..120)
	require.False(t, checkTempRange(0, -61, 30, &lo, &hi))
}

func TestParseAsTemperatureTemp10X(t *testing.T) {
	t.Parallel()

	attr := AtaSmartAttr{Type: AtaDeviceAttributeTypeTemp10X, ValueRaw: 250}
	val, low, hi, extra, err := attr.ParseAsTemperature()
	require.NoError(t, err)
	require.Equal(t, 25, val) // 250 / 10
	require.Equal(t, 0, low)
	require.Equal(t, 0, hi)
	require.Equal(t, 0, extra)
}

func TestIsGeneralPurposeLoggingCapable(t *testing.T) {
	t.Parallel()

	var id AtaIdentifyDevice

	// Neither word has the validity marker → false
	require.False(t, id.IsGeneralPurposeLoggingCapable())

	// CommandsSupported3 (word 84): set validity bits (15:14 = 0b01) but not GPL bit → false
	id.CommandsSupported3 = 0x4000 // bit 14 set, bit 15 clear, bit 5 clear
	require.False(t, id.IsGeneralPurposeLoggingCapable())

	// Set GPL bit (bit 5) as well → true
	id.CommandsSupported3 = 0x4020 // bits 14 and 5 set
	require.True(t, id.IsGeneralPurposeLoggingCapable())

	// CommandsEnabled3 (word 87) path: clear supported3 validity, set enabled3
	id.CommandsSupported3 = 0x0000
	id.CommandsEnabled3 = 0x4020
	require.True(t, id.IsGeneralPurposeLoggingCapable())

	// Enabled3 without GPL bit → false
	id.CommandsEnabled3 = 0x4000
	require.False(t, id.IsGeneralPurposeLoggingCapable())
}

func TestWWN(t *testing.T) {
	t.Parallel()

	id := AtaIdentifyDevice{
		WWNRaw: [4]uint16{0x0011, 0x2233, 0x4455, 0x6677},
	}
	require.Equal(t, uint64(0x0011223344556677), id.WWN())
}

func TestAtaIdentifyDeviceCapacity(t *testing.T) {
	t.Parallel()

	var id AtaIdentifyDevice

	// LBA not supported → all zeros.
	sectors, capacity, _, _, _ := id.Capacity()
	require.Equal(t, uint64(0), sectors)
	require.Equal(t, uint64(0), capacity)

	// LBA28 only, 512-byte sectors.
	id.Capabilities[0] = 0x0200 // bit 9 set: LBA supported
	id.CapacityLba28 = 1024
	sectors, capacity, lss, pss, _ := id.Capacity()
	require.Equal(t, uint64(1024), sectors)
	require.Equal(t, uint64(1024)*512, capacity)
	require.Equal(t, uint64(512), lss)
	require.Equal(t, uint64(512), pss)

	// LBA48 takes precedence when >= LBA28.
	id.CommandsSupported2 = 0x0400 // bit 10: 48-bit address feature set
	id.CapacityLba48 = 2048
	sectors, capacity, _, _, _ = id.Capacity()
	require.Equal(t, uint64(2048), sectors)
	require.Equal(t, uint64(2048)*512, capacity)
}
