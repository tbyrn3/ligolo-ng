//go:build linux

package tun

import (
	"net"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/link/tun"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type TunInterface struct {
	LinkEP stack.LinkEndpoint
	fd     int
	name   string
}

func New(tunName string) (*TunInterface, error) {
	tunIface := TunInterface{}
	iface, err := net.InterfaceByName(tunName)
	if err != nil {
		return nil, err
	}
	mtu := uint32(iface.MTU)

	fd, err := tun.Open(tunName)
	if err != nil {
		return nil, err
	}
	tunIface.fd = fd
	tunIface.name = tunName

	linkEP, err := fdbased.New(&fdbased.Options{FDs: []int{fd}, MTU: mtu})
	if err != nil {
		return nil, err
	}
	tunIface.LinkEP = linkEP

	return &tunIface, nil
}

func (t TunInterface) Name() (string, error) {
	return t.name, nil
}

func (t *TunInterface) Close() error {
	return unix.Close(t.fd)
}
