package test

import (
	"testing"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	path := "/dev/nvme0n1"

	dev, err := smart.Open(path)
	require.NoError(t, err)
	defer dev.Close()

	require.IsType(t, (*smart.NVMeDevice)(nil), dev)
}

func TestGenericAttributesNVMe(t *testing.T) {
	path := "/dev/nvme0n1"

	dev, err := smart.Open(path)
	require.NoError(t, err)
	defer dev.Close()

	a, err := dev.ReadGenericAttributes()
	require.NoError(t, err)
	// expected temperature is in range of 20-60C
	require.Less(t, uint64(20), a.Temperature)
	require.Greater(t, uint64(60), a.Temperature)
	require.Equal(t, uint64(0), a.PowerCycles)
	require.Equal(t, uint64(0), a.PowerOnHours)
	require.Equal(t, uint64(3), a.Read)
	require.Equal(t, uint64(0), a.Written)
}

func TestGenericAttributesSata(t *testing.T) {
	path := "/dev/sdc"

	dev, err := smart.Open(path)
	require.NoError(t, err)
	defer dev.Close()

	a, err := dev.ReadGenericAttributes()
	require.NoError(t, err)
	// expected temperature is in range of 20-60C
	require.Less(t, uint64(20), a.Temperature)
	require.Greater(t, uint64(60), a.Temperature)
	require.Equal(t, uint64(0), a.PowerCycles)
	require.Equal(t, uint64(1), a.PowerOnHours)
	require.Equal(t, uint64(0), a.Read)    // QEMU does not report read LBA count
	require.Equal(t, uint64(0), a.Written) // QEMU does not report written LBA count
}
