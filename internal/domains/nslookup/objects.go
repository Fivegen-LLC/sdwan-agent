package nslookup

import (
	"fmt"
)

type hostPair struct {
	fqdn string
	ip   string
}

type hostPairs []hostPair

func newHostPair(fqdn, ip string) hostPair {
	return hostPair{
		fqdn: fqdn,
		ip:   ip,
	}
}

func (d hostPair) buildHostLine() string {
	return fmt.Sprintf("%s %s", d.ip, d.fqdn)
}
