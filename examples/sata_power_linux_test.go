package test

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

// spinUp forces a real media access so the drive leaves PM2:Standby.
//
// iflag=direct is essential: a buffered read of a sector that is already in
// the page cache is served entirely from RAM and never reaches the drive, so
// it does NOT spin it up.
func spinUp(t *testing.T, path string) {
	t.Helper()

	out, err := exec.Command("dd", "if="+path, "of=/dev/null",
		"bs=512", "count=1", "iflag=direct").CombinedOutput()
	fmt.Println(string(out))
	require.NoError(t, err)
}

func TestSataPower(t *testing.T) {
	path := "/dev/sda"

	dev, err := smart.OpenSata(path)
	require.NoError(t, err)
	defer dev.Close()

	// CHECK POWER MODE never spins up a spun-down drive, so it is safe to
	// call at any time. Cross-check the reported state with
	// `smartctl --nocheck=standby -a <path>` or `hdparm -C <path>`.
	mode, err := dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("Power mode: ", mode)
	require.Contains(t, []smart.AtaPowerMode{
		smart.AtaPowerModeStandby,
		smart.AtaPowerModeStandbyY,
		smart.AtaPowerModeIdle,
		smart.AtaPowerModeIdleA,
		smart.AtaPowerModeIdleB,
		smart.AtaPowerModeIdleC,
		smart.AtaPowerModeActiveOrIdle,
	}, mode)

	// EPC support/enabled bits from IDENTIFY (words 119/120 bit 7).
	i, err := dev.Identify()
	require.NoError(t, err)
	fmt.Println("EPC supported: ", i.EpcSupported())
	fmt.Println("EPC enabled:   ", i.EpcEnabled())
	fmt.Println("APM supported: ", i.ApmSupported())

	// --- Standby: spin down, verify, spin back up ---
	err = dev.Standby()
	require.NoError(t, err)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After Standby(): ", mode)
	require.Equal(t, smart.AtaPowerModeStandby, mode)

	// Spin the drive back up with a media access.
	spinUp(t, path)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After spin-up: ", mode)
	require.Contains(t, []smart.AtaPowerMode{
		smart.AtaPowerModeActiveOrIdle,
		smart.AtaPowerModeIdle,
		smart.AtaPowerModeIdleA,
	}, mode, "drive still in standby after direct media read: read likely served from cache")

	// --- Idle: enter Idle_a, platters keep spinning ---
	err = dev.Idle()
	require.NoError(t, err)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After Idle(): ", mode)
	require.Contains(t, []smart.AtaPowerMode{
		smart.AtaPowerModeIdle,
		smart.AtaPowerModeIdleA,
		smart.AtaPowerModeActiveOrIdle,
	}, mode)

	// --- Standby timer: 10s → raw value 2 ---
	err = dev.SetStandbyTimer(10 * time.Second)
	require.NoError(t, err)

	// SetStandbyTimer also spins the drive down (STANDBY side effect).
	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After SetStandbyTimer(10s): ", mode)
	require.Equal(t, smart.AtaPowerModeStandby, mode)

	// Spin back up before the timer test below.
	spinUp(t, path)

	// --- Disable the timer via the raw variant ---
	err = dev.SetStandbyTimerRaw(0)
	require.NoError(t, err)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After SetStandbyTimerRaw(0): ", mode)
	require.Equal(t, smart.AtaPowerModeStandby, mode)

	// --- Idle timer: sets the timer without spinning down ---
	spinUp(t, path)

	err = dev.SetIdleTimer(10 * time.Second)
	require.NoError(t, err)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After SetIdleTimer(10s): ", mode)
	require.Contains(t, []smart.AtaPowerMode{
		smart.AtaPowerModeIdle,
		smart.AtaPowerModeIdleA,
		smart.AtaPowerModeActiveOrIdle,
	}, mode)

	// --- Raw idle timer variant: disable the timer without spinning down ---
	err = dev.SetIdleTimerRaw(0)
	require.NoError(t, err)

	mode, err = dev.CheckPowerMode()
	require.NoError(t, err)
	fmt.Println("After SetIdleTimerRaw(0): ", mode)
	require.Contains(t, []smart.AtaPowerMode{
		smart.AtaPowerModeIdle,
		smart.AtaPowerModeIdleA,
		smart.AtaPowerModeActiveOrIdle,
	}, mode)

	// --- APM level round-trip ---
	if i.ApmSupported() {
		err = dev.SetAPMLevel(128)
		require.NoError(t, err)

		// Word 91 is only visible via a fresh IDENTIFY DEVICE command.
		i, err = dev.Identify()
		require.NoError(t, err)
		require.True(t, i.ApmEnabled())
		require.Equal(t, uint8(128), i.CurrentApmLevel())

		// Disable APM again.
		err = dev.DisableAPM()
		require.NoError(t, err)

		i, err = dev.Identify()
		require.NoError(t, err)
		require.False(t, i.ApmEnabled())
	} else {
		fmt.Println("APM not supported: skipping APM level round-trip")
	}

	// --- IdleUnload: heads retract, drive stays in Idle ---
	if i.UnloadSupported() {
		err = dev.IdleUnload()
		require.NoError(t, err)

		mode, err = dev.CheckPowerMode()
		require.NoError(t, err)
		fmt.Println("After IdleUnload(): ", mode)
		require.Contains(t, []smart.AtaPowerMode{
			smart.AtaPowerModeIdle,
			smart.AtaPowerModeIdleA,
			smart.AtaPowerModeActiveOrIdle,
		}, mode)
	} else {
		fmt.Println("Unload not supported: skipping IdleUnload()")
	}

	// --- SetPowerMode: EPC-only conditions need EPC support ---
	if i.EpcSupported() {
		err = dev.SetPowerMode(smart.AtaPowerModeIdleA)
		require.NoError(t, err)

		mode, err = dev.CheckPowerMode()
		require.NoError(t, err)
		fmt.Println("After SetPowerMode(IdleA): ", mode)
		require.Equal(t, smart.AtaPowerModeIdleA, mode)
	} else {
		fmt.Println("EPC not supported: skipping SetPowerMode(IdleA)")
	}

	// ActiveOrIdle is not settable by any command.
	err = dev.SetPowerMode(smart.AtaPowerModeActiveOrIdle)
	require.Error(t, err)

	// --- SLEEP: the drive becomes unusable until a reset. Run LAST. ---
	err = dev.Sleep()
	require.NoError(t, err)
	fmt.Println("Drive is now in PM3:Sleep; recover with a sysfs rescan, replug, or reboot.")
}
