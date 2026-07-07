package test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// requireFdFlag locates the process's open descriptor(s) for path and asserts
// that (flags & mask) == want, where flags are read via fcntl(fd, cmd).
//
// It works for both fcntl flag namespaces:
//   - descriptor flags:  cmd = unix.F_GETFD, e.g. FD_CLOEXEC
//   - access mode:       cmd = unix.F_GETFL, mask = O_ACCMODE, want = O_RDONLY
//     (matched by value, since O_RDONLY is 0 and can't be tested as a bit)
//
// The smart.go device types keep their fd unexported, so we find it by scanning
// /proc/self/fd for the symlink that resolves to path.
func requireFdFlag(t *testing.T, path string, cmd, mask, want int, desc string) {
	t.Helper()

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	found := false
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil || target != path {
			continue
		}

		fd, err := strconv.Atoi(e.Name())
		require.NoError(t, err)

		flags, err := unix.FcntlInt(uintptr(fd), cmd, 0)
		require.NoError(t, err)
		require.Equalf(t, want, flags&mask, "fd for %s: expected %s", path, desc)
		found = true
	}

	require.Truef(t, found, "no open fd found for %s", path)
}

// requireCloexec asserts the device fd for path has close-on-exec set, so the
// raw device fd is not leaked to child processes across execve.
func requireCloexec(t *testing.T, path string) {
	t.Helper()
	requireFdFlag(t, path, unix.F_GETFD, unix.FD_CLOEXEC, unix.FD_CLOEXEC, "FD_CLOEXEC set")
}

// requireReadOnly asserts the device fd for path was opened read-only.
func requireReadOnly(t *testing.T, path string) {
	t.Helper()
	requireFdFlag(t, path, unix.F_GETFL, unix.O_ACCMODE, unix.O_RDONLY, "O_RDONLY access mode")
}
