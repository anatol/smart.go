# smart.go

Smart.go is a pure Golang library to access disk low-level [S.M.A.R.T.](https://en.wikipedia.org/wiki/S.M.A.R.T.) information.
Smart.go tries to match functionality provided by [smartctl](https://www.smartmontools.org/) but with golang API.

Currently this library support SATA, SCSI and NVMe drives. Different drive types provide different set of monitoring information and API reflects it.

At this point the library works at Linux and partially at MacOSX. We are looking for help with porting it to other platforms.

## Example

Here is an example of code that demonstrates the library usage.

```go
// skip the error handling for more compact API example
dev, _ := smart.OpenNVMe("/dev/nvme0n1")
c, nss, _ := dev.Identify()
fmt.Println("Model number: ", c.ModelNumber())
fmt.Println("Serial number: ", c.SerialNumber())
fmt.Println("Size: ", c.Tnvmcap.Val[0])

// namespace #1
ns := nss[0]
fmt.Println("Namespace 1 utilization: ", ns.Nuse*ns.LbaSize())

sm, _ := dev.ReadSMART()
fmt.Println("Temperature: ", sm.Temperature, "K")
// PowerOnHours is reported as 128-bit value and represented by this library as an array of uint64
fmt.Println("Power-on hours: ", sm.PowerOnHours.Val[0])
fmt.Println("Power cycles: ", sm.PowerCycles.Val[0])
```

The output looks like
```text
Model number:  SAMSUNG MZVLB512HBJQ-000L7
Serial number:  S4ENNF0M741521
Size:  512110190592
Namespace 1 utilization:  387524902912
Temperature:  327 K
Power-on hours:  499
Power cycles:  1433
```

Here is an example of iterating over system's block devices:
```go
block, err := ghw.Block()
if err != nil {
  panic(err)
}
for _, disk := range block.Disks {
        dev, err := smart.Open("/dev/" + disk.Name)
        if err != nil {
            // some devices (like dmcrypt) do not support SMART interface
            fmt.Println(err)
            continue
        }
        defer dev.Close()

        switch sm := dev.(type) {
        case *smart.SataDevice:
            data, err := sm.ReadSMARTData()
            if attr, ok := data.Attrs[194]; ok { // attr.Name == "Temperature_Celsius"
                temp, min, max, overtempCounter, err := attr.ParseAsTemperature()
                // min/max/counter are optional
            }
        case *smart.ScsiDevice:
            _, _ = sm.Capacity()
        case *smart.NVMeDevice:
            _, _ = sm.ReadSMART()
        }
}
```

Reading generic SMART attributes.

smart.go provides API for easier access to the most commonly used device attributes.

```go
dev, err := smart.Open("/dev/nvme0n1")
require.NoError(t, err)
defer dev.Close()

a, err := dev.ReadGenericAttributes()
require.NoError(t, err)

fmt.Println("The temperature is ", a.Temperature) // in Celsius
fmt.Println("Read block count ", a.Read)
fmt.Println("Written block count ", a.Written)
fmt.Println("Power Cycles count ", a.PowerCycles)
fmt.Println("Power On Hours ", a.PowerOnHours)
```

SATA drives can report their current power state (whether the platters are spinning or the drive is in standby). The underlying ATA CHECK POWER MODE command never spins up a spun-down drive, so it is safe to call at any time.

```go
dev, err := smart.OpenSata("/dev/sda")
require.NoError(t, err)
defer dev.Close()

mode, err := dev.CheckPowerMode()
require.NoError(t, err)
fmt.Println("Power mode: ", mode) // "active or idle", "idle" or "standby"
```

SATA drives can also be put into a power state. The setters map to the ATA
power management commands; all of them return an error if the drive rejects
the command.

```go
// Spin down to Standby_z (hdparm -y equivalent). Does not flush the OS page
// cache - sync first if that matters.
err := dev.Standby()

// Enter Idle_a; platters keep spinning (hdparm --idle-immediate equivalent).
err = dev.Idle()

// Retract the heads to the ramp/landing zone; the drive stays spinning in
// Idle_a and reloads the heads on the next media access. Requires the Unload
// feature - check with Identify().UnloadSupported() first.
err := dev.IdleUnload()

// EPC-only conditions (Standby_y, Idle_a/b/c) via SET FEATURES; requires
// the Extended Power Conditions feature set (check with Identify().EpcSupported()).
err := dev.SetPowerMode(smart.AtaPowerModeIdleA)

// Standby timer: spin down after 10 seconds of inactivity. Also spins the
// drive down immediately (STANDBY command side effect). 0 disables the timer.
err := dev.SetStandbyTimer(10 * time.Second)

// Raw Table 52 timer value, for encodings outside the duration API
// (e.g. vendor-specific 0xfd). Same side effect as SetStandbyTimer.
err := dev.SetStandbyTimerRaw(0xfd)

// Idle timer: set the Standby timer without spinning down (IDLE command).
err := dev.SetIdleTimer(10 * time.Second)

// Raw Table 52 timer value via the IDLE command, for encodings outside the
// duration API (e.g. vendor-specific 0xfd). Same side effect as SetIdleTimer.
err := dev.SetIdleTimerRaw(0xfd)

// APM level 1..254 (1 = minimum power, 254 = maximum performance).
// APM and EPC are mutually exclusive: enabling APM disables EPC.
err := dev.SetAPMLevel(128)
err = dev.DisableAPM()
```

APM status comes from the IDENTIFY DEVICE data (no ATA command reads it), and
the level is only meaningful while APM is enabled; re-issue Identify() after
SetAPMLevel/DisableAPM to see the new value. The same data tells you which
optional features the drive implements, so skip commands for unsupported ones:

```go
i, err := dev.Identify()
require.NoError(t, err)
fmt.Println("APM supported:    ", i.ApmSupported())
fmt.Println("APM enabled:      ", i.ApmEnabled())
fmt.Println("APM level:        ", i.CurrentApmLevel()) // 1..254
fmt.Println("EPC supported:    ", i.EpcSupported())
fmt.Println("EPC enabled:      ", i.EpcEnabled())
fmt.Println("Unload supported: ", i.UnloadSupported())
```

`Sleep()` puts the drive into PM3:Sleep - the deepest state, from which **no
ATA command can wake it**. Recovery requires a hardware/software reset: a
sysfs rescan (`echo 1 > /sys/block/sdX/device/delete` then
`echo "- - -" > /sys/class/scsi_host/hostN/scan`), replugging the drive, or
a reboot. Use it only if you understand the recovery procedure.

### Credit
This project is inspired by https://github.com/dswarbrick/smart
