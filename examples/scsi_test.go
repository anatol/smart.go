package test

import (
	"bytes"
	"fmt"
	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
	"os/exec"
	"testing"
)

func TestScsi(t *testing.T) {
	path := "/dev/sdb"

	out, err := exec.Command("smartctl", "-a", path).CombinedOutput()
	fmt.Println(string(out))
	//require.NoError(t, err)

	dev, err := smart.OpenScsi(path)
	require.NoError(t, err)
	defer dev.Close()

	c, err := dev.Capacity()
	require.NoError(t, err)
	require.Equal(t, 0x2800000, int(c))

	i, err := dev.Inquiry()
	require.NoError(t, err)
	require.Equal(t, "QEMU", string(bytes.TrimSpace(i.VendorIdent[:])))

	s, err := dev.SerialNumber()
	require.NoError(t, err)
	require.Equal(t, "", s)
}
