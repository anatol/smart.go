package smart

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtaDeviceStatistics(t *testing.T) {
	t.Parallel()

	// Solid State Device Statistics page (07h), "Percentage Used Endurance
	// Indicator" at offset 008h.
	buf := make([]byte, 8*512)
	buf[AtaStatPercentageUsedEndurance] = 51     // value (low byte)
	buf[AtaStatPercentageUsedEndurance+7] = 0xC0 // flags: supported | valid
	s := &AtaDeviceStatistics{raw: buf}

	v, ok := s.Get(AtaStatPercentageUsedEndurance)
	require.True(t, ok)
	require.Equal(t, uint64(51), v)

	// Multi-byte value: the flags byte must be stripped from the result.
	gen := make([]byte, 8*512)
	gen[AtaStatPowerOnHours] = 0x34
	gen[AtaStatPowerOnHours+1] = 0x12
	gen[AtaStatPowerOnHours+7] = 0xC0 // supported | valid
	mv, ok := (&AtaDeviceStatistics{raw: gen}).Get(AtaStatPowerOnHours)
	require.True(t, ok)
	require.Equal(t, uint64(0x1234), mv)

	// Supported but not valid -> not present.
	inv := make([]byte, 8*512)
	inv[AtaStatPercentageUsedEndurance] = 51
	inv[AtaStatPercentageUsedEndurance+7] = 0x80 // supported only
	_, ok = (&AtaDeviceStatistics{raw: inv}).Get(AtaStatPercentageUsedEndurance)
	require.False(t, ok)

	// Out of range -> not present.
	_, ok = (&AtaDeviceStatistics{raw: make([]byte, 100)}).Get(AtaStatPercentageUsedEndurance)
	require.False(t, ok)
}
