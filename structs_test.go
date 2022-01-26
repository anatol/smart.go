package smart

import (
	"github.com/stretchr/testify/require"
	"testing"
	"unsafe"
)

func TestNvmeSizes(t *testing.T) {
	t.Parallel()

	var c NvmeIdentController
	require.Equal(t, 4096, int(unsafe.Sizeof(c)))
	require.Equal(t, 128, int(unsafe.Offsetof(c.Crdt1)))
	require.Equal(t, 516, int(unsafe.Offsetof(c.Nn)))

	var ns NvmeIdentNamespace
	require.Equal(t, 4096, int(unsafe.Sizeof(ns)))
}

func TestSataSizes(t *testing.T) {
	var d AtaIdentifyDevice
	require.Equal(t, 27*2, int(unsafe.Offsetof(d.ModelNumberRaw)))
	require.Equal(t, 75*2, int(unsafe.Offsetof(d.QueueDepth)))
	require.Equal(t, 108*2, int(unsafe.Offsetof(d.WWNRaw)))
	require.Equal(t, 222*2, int(unsafe.Offsetof(d.TransportMajor)))
	require.Equal(t, 512, int(unsafe.Sizeof(d)))
}
