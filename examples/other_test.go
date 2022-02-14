// go:build !linux
//go:build !linux
// +build !linux

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
