// go:build !linux&& !darwin
//go:build !linux && !darwin
// +build !linux,!darwin

package test

import (
	"testing"

	"github.com/anatol/smart.go"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	path := "/dev/nvme0n1"

	_, err := smart.Open(path)
	require.ErrorIs(t, err, smart.ErrOSUnsupported)
}
