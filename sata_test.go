package smart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseAsTemperatureRange(t *testing.T) {
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
