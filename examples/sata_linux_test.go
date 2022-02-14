package test

import (
	"bytes"
	"fmt"
	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
	"os/exec"
	"testing"
)

func TestSata(t *testing.T) {
	path := "/dev/sdc"

	out, err := exec.Command("smartctl", "-a", path).CombinedOutput()
	fmt.Println(string(out))
	//require.NoError(t, err)

	dev, err := smart.OpenSata(path)
	require.NoError(t, err)
	defer dev.Close()

	i, err := dev.Identify()
	require.NoError(t, err)
	fmt.Printf("%+v\n", i)
	require.Equal(t, "MQ0000 3", string(bytes.TrimSpace(i.SerialNumberRaw[:])))
	require.Equal(t, "QM00003", i.SerialNumber())
	require.Equal(t, [4]uint16{}, i.WWNRaw)
	require.Equal(t, uint64(0), i.WWN())

	page, err := dev.ReadSMARTData()
	require.NoError(t, err)
	fmt.Printf("%+v\n", page)

	if i.IsGeneralPurposeLoggingCapable() {
		dir, err := dev.ReadSMARTLogDirectory()
		require.NoError(t, err)
		fmt.Printf("%+v\n", dir)
	}

	log, err := dev.ReadSMARTErrorLogSummary()
	require.NoError(t, err)
	fmt.Printf("%+v\n", log)

	test, err := dev.ReadSMARTSelfTestLog()
	require.NoError(t, err)
	fmt.Printf("%+v\n", test)
}
