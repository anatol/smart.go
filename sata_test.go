package smart

import (
	"testing"

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
