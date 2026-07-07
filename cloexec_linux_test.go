package smart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestOpenSetsFlags verifies that device descriptors are opened read-only and
// with the O_CLOEXEC flag, so they are not leaked to child processes across
// execve.
func TestOpenSetsFlags(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fakedev")
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	dev, err := OpenNVMe(path)
	require.NoError(t, err)
	defer dev.Close()

	fdFlags, err := unix.FcntlInt(uintptr(dev.fd), unix.F_GETFD, 0)
	require.NoError(t, err)
	require.NotZero(t, fdFlags&unix.FD_CLOEXEC, "device fd must have FD_CLOEXEC set")

	statusFlags, err := unix.FcntlInt(uintptr(dev.fd), unix.F_GETFL, 0)
	require.NoError(t, err)
	require.Equal(t, unix.O_RDONLY, statusFlags&unix.O_ACCMODE, "device fd must be read-only")
}
