package test

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

func TestNVMe(t *testing.T) {
	path := "/dev/nvme0n1"

	out, err := exec.Command("smartctl", "-a", path).CombinedOutput()
	fmt.Println(string(out))
	require.NoError(t, err)

	dev, err := smart.OpenNVMe(path)
	require.NoError(t, err)
	defer dev.Close()

	c, ns, err := dev.Identify()
	require.NoError(t, err)

	require.Equal(t, 0x1b36, int(c.VendorID))
	require.Equal(t, 0x1af4, int(c.Ssvid))
	require.Equal(t, "smarttest", string(bytes.TrimSpace(c.SerialNumber[:])))
	require.Equal(t, "QEMU NVMe Ctrl", string(bytes.TrimSpace(c.ModelNumber[:])))
	require.Equal(t, 256, int(c.Nn))

	require.Len(t, ns, 1)
	require.Equal(t, 0x14000, int(ns[0].Nsze))

	sm, err := dev.ReadSMART()
	require.NoError(t, err)
	require.Less(t, uint16(300), sm.Temperature)
}
