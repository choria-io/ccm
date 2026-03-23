// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"
	"fmt"
	"net"

	"github.com/choria-io/ccm/model"
	gnet "github.com/shirou/gopsutil/v4/net"
)

func getNetworkFacts(ctx context.Context, opts *model.FactsConfig) map[string]any {
	networkFacts := map[string]any{
		"addresses":    []any{},
		"interfaces":   []any{},
		"default_ipv6": "",
		"default_ipv4": "",
	}

	if opts.NoNetworkFacts {
		return networkFacts
	}

	interfaces, err := gnet.InterfacesWithContext(ctx)
	if err == nil {
		networkFacts["interfaces"] = interfaces

		dflt4, _ := getDefaultInterface("udp4", "1.1.1.1:53")
		dflt6, _ := getDefaultInterface("udp6", "[2001:4860:4860::8888]:53")

		addresses := []any{}
		for _, i := range interfaces {
			for _, a := range i.Addrs {
				parsed, _, err := net.ParseCIDR(a.Addr)
				if err != nil || parsed.String() == "" || parsed.IsLinkLocalMulticast() || parsed.IsLinkLocalUnicast() || parsed.IsLoopback() || parsed.IsMulticast() || parsed.IsUnspecified() {
					continue
				}

				if parsed.To4() != nil {
					addresses = append(addresses, map[string]any{
						"interface": i.Name,
						"address":   parsed.String(),
						"ipv4":      true,
						"default":   dflt4 == i.Name,
					})
					if dflt4 == i.Name {
						networkFacts["default_ipv4"] = parsed.String()
					}
				} else if parsed.To16() != nil {
					addresses = append(addresses, map[string]any{
						"interface": i.Name,
						"address":   parsed.String(),
						"ipv4":      false,
						"default":   dflt6 == i.Name,
					})
					if dflt6 == i.Name {
						networkFacts["default_ipv6"] = parsed.String()
					}
				}
			}
		}
		networkFacts["addresses"] = addresses
	}

	return networkFacts
}

func getDefaultInterface(network string, address string) (string, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return "", fmt.Errorf("could not determine default route: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("could not list network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.Equal(localAddr.IP) {
				return iface.Name, nil
			}
		}
	}

	return "", fmt.Errorf("could not find interface for address %s", localAddr.IP)
}
