package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

func TestNVMe(t *testing.T) {
	path := "disk0"

	out, err := exec.Command("smartctl", "-a", path).CombinedOutput()
	fmt.Println(string(out))
	// require.NoError(t, err)  it fails at macosx because of GetLogPage()

	dev, err := smart.OpenNVMe(path)
	require.NoError(t, err)
	defer dev.Close()

	c, ns, err := dev.Identify()
	require.NoError(t, err)

	require.Equal(t, 0x106b, int(c.VendorID))
	require.Equal(t, 0x106b, int(c.Ssvid))
	require.Contains(t, c.ModelNumber(), "APPLE SSD")
	require.GreaterOrEqual(t, int(c.Nn), 1)

	require.NotEmpty(t, ns)
	require.LessOrEqual(t, len(ns), int(c.Nn))
	hasNonZeroSize := false
	for _, n := range ns {
		if n.Nsze > 0 {
			hasNonZeroSize = true
			break
		}
	}
	require.True(t, hasNonZeroSize)

	sm, err := dev.ReadSMART()
	require.NoError(t, err)
	require.Less(t, uint16(270), sm.Temperature)
	require.Greater(t, uint16(370), sm.Temperature)
}
