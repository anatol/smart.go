# smart.go

Smart.go is a pure Golang library to access disk low-level [S.M.A.R.T.](https://en.wikipedia.org/wiki/S.M.A.R.T.) information.
Smart.go tries to match functionality provided by [smartctl](https://www.smartmontools.org/) but with golang API.

Currently this library support SATA, SCSI and NVMe drives. Different drive types provide different set of monitoring information and API reflects it.

## Example

Here is an example of code that demonstrates the library usage.

```go
// skip the error handling for more compact API example
dev, _ := smart.OpenNVMe("/dev/nvme0n1")
c, nss, _ := dev.Identify()
fmt.Println("Model number: ", string(bytes.TrimSpace(c.ModelNumber[:])))
fmt.Println("Serial number: ", string(bytes.TrimSpace(c.SerialNumber[:])))
fmt.Println("Size: ", c.Tnvmcap.Val[0])

// namespace #1
ns := nss[0]
lbaSize := uint64(1) << ns.Lbaf[ns.Flbas&0xf].Ds
fmt.Println("Namespace 1 utilization: ", ns.Nuse*lbaSize)

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

### Credit
This project is inspired by https://github.com/dswarbrick/smart
