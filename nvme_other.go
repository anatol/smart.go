//+build !linux
//go:build !linux

package smart

func OpenNVMe(name string) (*NVMeDevice, error) {
	return nil, ErrOSUnsupported
}

func (d *NVMeDevice) Close() error {
	return ErrOSUnsupported
}

func (d *NVMeDevice) Identify() (*NvmeIdentController, []NvmeIdentNamespace, error) {
	return nil, nil, ErrOSUnsupported
}

func (d *NVMeDevice) ReadSMART() (*NvmeSMARTLog, error) {
	return nil, ErrOSUnsupported
}
