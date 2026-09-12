//go:build !linux

package smart

import "time"

func OpenSata(name string) (*SataDevice, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) Close() error {
	return ErrOSUnsupported
}

func (d *SataDevice) Identify() (*AtaIdentifyDevice, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) readSMARTLog(logPage uint8) ([]byte, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) readSMARTData() (*AtaSmartPageRaw, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) ReadSMARTLogDirectory() (*AtaSmartLogDirectory, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) ReadSMARTErrorLogSummary() (*AtaSmartErrorLogSummary, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) ReadSMARTSelfTestLog() (*AtaSmartSelfTestLog, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) readSMARTThresholds() (*AtaSmartThresholdsPageRaw, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) ReadStatistics() (*AtaDeviceStatistics, error) {
	return nil, ErrOSUnsupported
}

func (d *SataDevice) CheckPowerMode() (AtaPowerMode, error) {
	return 0, ErrOSUnsupported
}

func (d *SataDevice) SetPowerMode(mode AtaPowerMode) error {
	return ErrOSUnsupported
}

func (d *SataDevice) SetStandbyTimer(timeout time.Duration) error {
	return ErrOSUnsupported
}

func (d *SataDevice) SetStandbyTimerRaw(raw byte) error {
	return ErrOSUnsupported
}

func (d *SataDevice) SetIdleTimer(timeout time.Duration) error {
	return ErrOSUnsupported
}

func (d *SataDevice) SetIdleTimerRaw(raw byte) error {
	return ErrOSUnsupported
}

func (d *SataDevice) SetAPMLevel(level uint8) error {
	return ErrOSUnsupported
}

func (d *SataDevice) DisableAPM() error {
	return ErrOSUnsupported
}

func (d *SataDevice) IdleUnload() error {
	return ErrOSUnsupported
}

func (d *SataDevice) Sleep() error {
	return ErrOSUnsupported
}

func (d *SataDevice) Standby() error {
	return ErrOSUnsupported
}

func (d *SataDevice) Idle() error {
	return ErrOSUnsupported
}
