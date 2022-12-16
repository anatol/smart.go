package test

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

func TestSata(t *testing.T) {
	path := "/dev/sdc"

	out, err := exec.Command("smartctl", "-a", path).CombinedOutput()
	fmt.Println(string(out))
	// require.NoError(t, err)

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

	thr, err := dev.ReadSMARTThresholds()
	require.NoError(t, err)
	require.Equal(t, 1, int(thr.Revnumber))

	// ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
	//   1 Raw_Read_Error_Rate     0x0003   100   100   006    Pre-fail  Always       -       0
	//   3 Spin_Up_Time            0x0003   100   100   000    Pre-fail  Always       -       16
	//   4 Start_Stop_Count        0x0002   100   100   020    Old_age   Always       -       100
	//   5 Reallocated_Sector_Ct   0x0003   100   100   036    Pre-fail  Always       -       0
	//   9 Power_On_Hours          0x0003   100   100   000    Pre-fail  Always       -       1
	//  12 Power_Cycle_Count       0x0003   100   100   000    Pre-fail  Always       -       0
	// 190 Airflow_Temperature_Cel 0x0003   069   069   050    Pre-fail  Always       -       31 (Min/Max 31/31)
	for id, a := range page.Attrs {
		switch id {
		case 1:
			require.Equal(t, "Raw_Read_Error_Rate", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 0, int(a.ValueRaw))
			require.Equal(t, 6, int(thr.Thresholds[1]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 3:
			require.Equal(t, "Spin_Up_Time", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 16, int(a.ValueRaw))
			require.Equal(t, 0, int(thr.Thresholds[3]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 4:
			require.Equal(t, "Start_Stop_Count", a.Name)
			require.Equal(t, 0x0002, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 100, int(a.ValueRaw))
			require.Equal(t, 20, int(thr.Thresholds[4]))
			require.Equal(t, false, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 5:
			require.Equal(t, "Reallocated_Sector_Ct", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 0, int(a.ValueRaw))
			require.Equal(t, 36, int(thr.Thresholds[5]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 9:
			require.Equal(t, "Power_On_Hours", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 1, int(a.ValueRaw))
			require.Equal(t, 0, int(thr.Thresholds[9]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 12:
			require.Equal(t, "Power_Cycle_Count", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 100, int(a.Current))
			require.Equal(t, 100, int(a.Worst))
			require.Equal(t, 0, int(a.ValueRaw))
			require.Equal(t, 0, int(thr.Thresholds[12]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		case 190:
			require.Equal(t, "Airflow_Temperature_Cel", a.Name)
			require.Equal(t, 0x0003, int(a.Flags))
			require.Equal(t, 69, int(a.Current))
			require.Equal(t, 69, int(a.Worst))
			val, low, high, counter, err := a.ParseAsTemperature()
			require.NoError(t, err)
			require.Equal(t, 31, val)
			require.Equal(t, 31, low)
			require.Equal(t, 31, high)
			require.Equal(t, 0, counter) // not supported at this drive
			require.Equal(t, 50, int(thr.Thresholds[190]))
			require.Equal(t, true, a.AttributeFlagsPrefailure())
			require.Equal(t, true, a.AttributeFlagsOnline())
		}
	}

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
