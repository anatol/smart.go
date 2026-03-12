# smart.go

A pure-Go library for reading [S.M.A.R.T.](https://en.wikipedia.org/wiki/S.M.A.R.T.) (Self-Monitoring, Analysis and Reporting Technology) data from storage devices. The library provides a unified API inspired by [smartctl](https://www.smartmontools.org/) and supports SATA, SCSI, and NVMe drives.

## Platform support

| Platform | SATA | SCSI | NVMe |
|---|---|---|---|
| Linux | ✅ | ✅ | ✅ |
| macOS | ❌ | ❌ | partial (stub) |
| Other | ❌ | ❌ | ❌ |

Contributions to expand platform coverage are welcome.

## Installation

```
go get github.com/anatol/smart.go
```

## Quick start

### Auto-detect device type

`Open` probes the device path and returns the most specific concrete type it can
identify (NVMe → SATA → SCSI). Use a type switch to access device-specific APIs.

```go
package main

import (
    "fmt"
    "log"

    smart "github.com/anatol/smart.go"
)

func main() {
    dev, err := smart.Open("/dev/sda")
    if err != nil {
        log.Fatal(err)
    }
    defer dev.Close()

    attrs, err := dev.ReadGenericAttributes()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Type:          ", dev.Type())
    fmt.Println("Temperature:   ", attrs.Temperature, "°C")
    fmt.Println("Power-on hours:", attrs.PowerOnHours)
    fmt.Println("Power cycles:  ", attrs.PowerCycles)
    fmt.Println("Data read:     ", attrs.Read, "LBA")
    fmt.Println("Data written:  ", attrs.Written, "LBA")
}
```

### Iterating over all block devices

```go
import "github.com/jaypipes/ghw"

block, err := ghw.Block()
if err != nil {
    log.Fatal(err)
}

for _, disk := range block.Disks {
    dev, err := smart.Open("/dev/" + disk.Name)
    if err != nil {
        // Devices like dm-crypt volumes do not expose a SMART interface.
        fmt.Println("skipping", disk.Name, ":", err)
        continue
    }
    defer dev.Close()

    switch d := dev.(type) {
    case *smart.NVMeDevice:
        sm, _ := d.ReadSMART()
        fmt.Printf("%s  temp=%dK  poweron=%d h\n",
            disk.Name, sm.Temperature, sm.PowerOnHours.Val[0])

    case *smart.SataDevice:
        data, _ := d.ReadSMARTData()
        if attr, ok := data.Attrs[194]; ok {
            temp, _, _, _, _ := attr.ParseAsTemperature()
            fmt.Printf("%s  temp=%d°C\n", disk.Name, temp)
        }

    case *smart.ScsiDevice:
        capacity, _ := d.Capacity()
        fmt.Printf("%s  capacity=%d bytes\n", disk.Name, capacity)
    }
}
```

---

## API reference

### Generic API

These types and functions work across all device types.

#### `smart.Open(path string) (Device, error)`

Opens the device at `path` and auto-detects its type by probing NVMe, then SATA,
then SCSI in order. Returns the first type that succeeds, or a combined error if
none match.

#### `Device` interface

```go
type Device interface {
    Type() string   // "nvme", "sata", or "scsi"
    Close() error

    // ReadGenericAttributes returns commonly used attributes normalised to
    // the same units regardless of the underlying device type. This API is
    // experimental and subject to change.
    ReadGenericAttributes() (*GenericAttributes, error)
}
```

#### `GenericAttributes`

```go
type GenericAttributes struct {
    Temperature  uint64  // Device temperature in Celsius
    Read         uint64  // Data units read (LBA)
    Written      uint64  // Data units written (LBA)
    PowerOnHours uint64  // Cumulative powered-on time in hours
    PowerCycles  uint64  // Number of power on/off cycles
}
```

---

### NVMe devices

#### Opening

```go
dev, err := smart.OpenNVMe("/dev/nvme0")
```

#### `(*NVMeDevice).Identify() (*NvmeIdentController, []NvmeIdentNamespace, error)`

Returns the controller identify structure and a slice of non-empty namespace
identify structures. The controller structure contains the device model, serial
number, firmware revision, and many capability flags defined by the NVMe spec.

```go
controller, namespaces, err := dev.Identify()
fmt.Println("Model: ", controller.ModelNumber())
fmt.Println("Serial:", controller.SerialNumber())
fmt.Println("FW:    ", controller.FirmwareRev())

for i, ns := range namespaces {
    fmt.Printf("  Namespace %d: size=%d bytes  utilization=%d bytes\n",
        i+1, ns.Nsze*ns.LbaSize(), ns.Nuse*ns.LbaSize())
}
```

#### `(*NVMeDevice).ReadSMART() (*NvmeSMARTLog, error)`

Reads the NVMe SMART / Health Information Log Page (log ID 0x02). Key fields:

| Field | Type | Description |
|---|---|---|
| `CritWarning` | `uint8` | Bitmask of critical warnings (spare low, temp, degraded, read-only, volatile backup) |
| `Temperature` | `uint16` | Composite temperature in **Kelvin** |
| `AvailSpare` | `uint8` | Available spare capacity as a percentage |
| `SpareThresh` | `uint8` | Spare threshold below which a warning is raised |
| `PercentUsed` | `uint8` | Estimated percentage of device life used |
| `DataUnitsRead` | `Uint128` | Units of 512,000 bytes read from the host |
| `DataUnitsWritten` | `Uint128` | Units of 512,000 bytes written by the host |
| `PowerCycles` | `Uint128` | Number of power cycles |
| `PowerOnHours` | `Uint128` | Cumulative power-on time in hours |
| `UnsafeShutdowns` | `Uint128` | Number of unsafe (non-graceful) shutdowns |
| `MediaErrors` | `Uint128` | Number of unrecovered media and data integrity errors |
| `WarningTempTime` | `uint32` | Minutes above the warning composite temperature |
| `CritCompTime` | `uint32` | Minutes above the critical composite temperature |
| `TempSensor` | `[8]uint16` | Individual temperature sensor readings in Kelvin |

> **Note:** NVMe reports temperature in Kelvin. Subtract 273 for Celsius.

```go
sm, err := dev.ReadSMART()
fmt.Printf("Temperature: %d K (%d °C)\n", sm.Temperature, int(sm.Temperature)-273)
fmt.Printf("Power-on hours: %d\n", sm.PowerOnHours.Val[0])
fmt.Printf("Power cycles: %d\n", sm.PowerCycles.Val[0])
fmt.Printf("Media errors: %d\n", sm.MediaErrors.Val[0])
fmt.Printf("Percentage used: %d%%\n", sm.PercentUsed)
```

#### `Uint128`

128-bit values (power-on hours, data written, etc.) are represented as:

```go
type Uint128 struct {
    Val [2]uint64  // Val[0] is the lower 64 bits; Val[1] is the upper 64 bits
}
```

For drives with values that fit in 64 bits, `Val[0]` is sufficient.

---

### SATA devices

#### Opening

```go
dev, err := smart.OpenSata("/dev/sda")
```

`OpenSata` verifies the device is a SATA drive (by checking the SCSI VPD vendor
identifier) and automatically looks up the device model in the built-in attribute
database.

#### `(*SataDevice).Identify() (*AtaIdentifyDevice, error)`

Issues an ATA IDENTIFY DEVICE command and returns the 512-byte response parsed
into `AtaIdentifyDevice`. Key accessor methods:

```go
id, err := dev.Identify()
fmt.Println("Model:    ", id.ModelNumber())
fmt.Println("Serial:   ", id.SerialNumber())
fmt.Println("Firmware: ", id.FirmwareRevision())
fmt.Printf ("WWN:       %016x\n", id.WWN())

sectors, capacity, lsSize, psSize, lsOffset := id.Capacity()
fmt.Printf("Capacity: %d bytes (%d sectors, %d-byte logical / %d-byte physical)\n",
    capacity, sectors, lsSize, psSize)
fmt.Println("GPL capable:", id.IsGeneralPurposeLoggingCapable())
```

#### `(*SataDevice).ReadSMARTData() (*AtaSmartPage, error)`

Returns all 30 SMART attribute slots in a map keyed by attribute ID. Unknown
attributes (not present in the built-in database) have an empty `Name` and a
zero `Type`.

```go
data, err := dev.ReadSMARTData()
for id, attr := range data.Attrs {
    fmt.Printf("  [%3d] %-35s  current=%3d  worst=%3d  raw=%d\n",
        id, attr.Name, attr.Current, attr.Worst, attr.ValueRaw)
}
```

**Common attribute IDs:**

| ID | Name | Description |
|---|---|---|
| 1 | `Raw_Read_Error_Rate` | Rate of hardware read errors |
| 5 | `Reallocated_Sector_Ct` | Count of reallocated sectors |
| 9 | `Power_On_Hours` | Total powered-on hours |
| 12 | `Power_Cycle_Count` | Total power on/off cycles |
| 177 | `Wear_Leveling_Count` | (SSD) Wear leveling count |
| 190 | `Airflow_Temperature_Cel` | Airflow temperature |
| 194 | `Temperature_Celsius` | Drive temperature |
| 197 | `Current_Pending_Sector` | Number of unstable sectors |
| 198 | `Offline_Uncorrectable` | Sectors that could not be corrected |
| 241 | `Total_LBAs_Written` | Total LBAs written |
| 242 | `Total_LBAs_Read` | Total LBAs read |

#### `AtaSmartAttr` — parsing helpers

Each `AtaSmartAttr` exposes:

```go
type AtaSmartAttr struct {
    Id          uint8
    Flags       uint16    // AtaAttributeFlagPrefailure | AtaAttributeFlagOnline
    Current     uint8     // Normalized current value (device-dependent scale)
    Worst       uint8     // Worst normalized value ever recorded
    VendorBytes [6]byte   // Raw storage bytes (interpreted by Type)
    Name        string    // Human-readable name from attribute database
    Type        int       // One of AtaDeviceAttributeType* constants
    ValueRaw    uint64    // Decoded raw value
}
```

**`(AtaSmartAttr).ParseAsDuration() (time.Duration, error)`**

Parses `ValueRaw` as a duration for time-based attributes
(`Power_On_Hours`, `Power_On_Minutes`, etc.).

```go
attr := data.Attrs[9] // Power_On_Hours
d, err := attr.ParseAsDuration()
fmt.Println("Power-on time:", d)
```

**`(AtaSmartAttr).ParseAsTemperature() (val, low, hi, overTempCount int, err error)`**

Parses temperature attributes. Returns the current temperature in Celsius.
`low` and `hi` are the lifetime min/max (supported by some drives). `overTempCount`
is an over-temperature event counter (supported by some drives). Unused optional
values are returned as zero.

```go
attr := data.Attrs[194] // Temperature_Celsius
temp, min, max, overCount, err := attr.ParseAsTemperature()
fmt.Printf("Temperature: %d°C  (min=%d, max=%d, over-temp events=%d)\n",
    temp, min, max, overCount)
```

**Attribute flags:**

```go
const (
    AtaAttributeFlagPrefailure = 1 << 0  // pre-failure; if cleared, old-age attribute
    AtaAttributeFlagOnline     = 1 << 1  // collected during normal operation; if cleared, offline only
)
```

#### `(*SataDevice).ReadSMARTThresholds() (*AtaSmartThresholdsPage, error)`

Returns the failure threshold for each attribute as a `map[uint8]uint8` keyed by
attribute ID. An attribute's `Current` value falling below its threshold signals
an imminent failure.

```go
thresholds, err := dev.ReadSMARTThresholds()
for id, attr := range data.Attrs {
    threshold := thresholds.Thresholds[id]
    failing := attr.Current < threshold
    fmt.Printf("  [%3d] %-35s  current=%d  threshold=%d  failing=%v\n",
        id, attr.Name, attr.Current, threshold, failing)
}
```

#### Log reading

```go
dir, err := dev.ReadSMARTLogDirectory()   // log page 0x00 — list of available log pages
errLog, err := dev.ReadSMARTErrorLogSummary()  // log page 0x01 — last 5 errors
selfTest, err := dev.ReadSMARTSelfTestLog()    // log page 0x06 — self-test history
```

---

### SCSI devices

#### Opening

```go
dev, err := smart.OpenScsi("/dev/sdb")
```

`OpenScsi` rejects devices that present as SATA (they are handled by
`OpenSata`) and rejects anything other than a Direct Access Block Device.

#### `(*ScsiDevice).Inquiry() (*ScsiInquiry, error)`

Issues a standard SCSI INQUIRY command. Returns vendor, product, and revision
strings along with the peripheral qualifier/type byte.

#### `(*ScsiDevice).SerialNumber() (string, error)`

Reads the Unit Serial Number via INQUIRY VPD page 0x80.

#### `(*ScsiDevice).Capacity() (uint64, error)`

Issues a READ CAPACITY(10) command and returns the total device capacity in bytes.

```go
capacity, err := dev.Capacity()
fmt.Printf("Capacity: %.2f GiB\n", float64(capacity)/(1<<30))
```

---

## Attribute database

SATA SMART attribute IDs are vendor-defined and their meaning varies across
device families. smart.go bundles a database derived from
[smartmontools](https://www.smartmontools.org/) that maps (model regex, firmware
regex) pairs to attribute name and type overrides.

The database is generated from `generator/ata_attributes.go` and written to
`ata_device_database.go`. To regenerate it:

```
go generate
```

When a device matches an entry in the database, its attribute overrides are
merged on top of the defaults. When no match is found, the default attribute
table is used and unknown attributes get empty names and a zero type.

---

## Contributing

Bug reports and pull requests are welcome. When adding support for a new
platform, see the existing `*_linux.go` / `*_other.go` pairs for the expected
file layout.

## Credits

Inspired by [dswarbrick/smart](https://github.com/dswarbrick/smart).

Attribute database derived from [smartmontools](https://www.smartmontools.org/).
