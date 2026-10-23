package smart

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux ioctl command encoding (see asm-generic/ioctl.h).
// A 32-bit ioctl number is split into four fields packed from LSB to MSB:
//
//	[7:0]   number   (8 bits) – command index within the type group
//	[15:8]  type     (8 bits) – magic byte identifying the subsystem (e.g. 'N' for NVMe)
//	[29:16] size     (14 bits) – size of the argument struct in bytes
//	[31:30] direction (2 bits) – data transfer direction (none/write/read)
const (
	directionNone  = 0
	directionWrite = 1
	directionRead  = 2

	numberBits    = 8
	typeBits      = 8
	sizeBits      = 14
	directionBits = 2

	numberMask    = (1 << numberBits) - 1
	typeMask      = (1 << typeBits) - 1
	sizeMask      = (1 << sizeBits) - 1
	directionMask = (1 << directionBits) - 1

	numberShift    = 0
	typeShift      = numberShift + numberBits
	sizeShift      = typeShift + typeBits
	directionShift = sizeShift + sizeBits
)

// ioc calculates the ioctl command for the specified direction, type, number and size
func ioc(dir, t, nr, size uintptr) uintptr {
	return (dir << directionShift) | (t << typeShift) | (nr << numberShift) | (size << sizeShift)
}

// ior calculates the ioctl command for a read-ioctl of the specified type, number and size
func ior(t, nr, size uintptr) uintptr {
	return ioc(directionRead, t, nr, size)
}

// iow calculates the ioctl command for a write-ioctl of the specified type, number and size
func iow(t, nr, size uintptr) uintptr {
	return ioc(directionWrite, t, nr, size)
}

// iowr calculates the ioctl command for a read/write-ioctl of the specified type, number and size
func iowr(t, nr, size uintptr) uintptr {
	return ioc(directionWrite|directionRead, t, nr, size)
}

// ioctl executes an ioctl command on the specified file descriptor.
//
// ptr is an unsafe.Pointer (not a uintptr) on purpose: it is converted to uintptr
// inside the unix.Syscall call expression, which is the only form where the compiler
// keeps the pointed-to object alive and valid for the duration of the call
// (see rule (4) of the unsafe.Pointer documentation).
func ioctl(fd, cmd uintptr, ptr unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, cmd, uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}

// heapBytes returns a zeroed byte slice that is always allocated on the heap.
//
// The kernel gets buffer addresses as plain integers inside command structs, which
// the compiler does not track: a stack buffer could be moved when the goroutine stack
// grows, leaving the kernel with a stale address. Heap objects never move.
// Converting a pointer to uintptr does not make the object escape, but returning it
// from a function that cannot be inlined does.
//
//go:noinline
func heapBytes(n int) []byte {
	return make([]byte, n)
}
