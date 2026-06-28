package smart

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtaDeviceStatistics(t *testing.T) {
	t.Parallel()

	// Solid State Device Statistics page (07h), "Percentage Used Endurance
	// Indicator" at offset 008h.
	const ssdEndurance = 0x07*512 + 0x008

	buf := make([]byte, 8*512)
	buf[ssdEndurance] = 51     // value (low byte)
	buf[ssdEndurance+7] = 0xC0 // flags: supported | valid
	s := &AtaDeviceStatistics{raw: buf}

	pct, ok := s.PercentUsedEndurance()
	require.True(t, ok)
	require.Equal(t, uint8(51), pct)

	v, ok := s.Get(0x07, 0x008)
	require.True(t, ok)
	require.Equal(t, uint64(51), v)

	// Multi-byte value: the flags byte must be stripped from the result.
	const general = 0x01*512 + 0x010
	gen := make([]byte, 8*512)
	gen[general] = 0x34
	gen[general+1] = 0x12
	gen[general+7] = 0xC0 // supported | valid
	mv, ok := (&AtaDeviceStatistics{raw: gen}).Get(0x01, 0x010)
	require.True(t, ok)
	require.Equal(t, uint64(0x1234), mv)

	// Supported but not valid -> not present.
	inv := make([]byte, 8*512)
	inv[ssdEndurance] = 51
	inv[ssdEndurance+7] = 0x80 // supported only
	_, ok = (&AtaDeviceStatistics{raw: inv}).PercentUsedEndurance()
	require.False(t, ok)

	// Out of range -> not present.
	_, ok = (&AtaDeviceStatistics{raw: make([]byte, 100)}).Get(0x07, 0x008)
	require.False(t, ok)
}
