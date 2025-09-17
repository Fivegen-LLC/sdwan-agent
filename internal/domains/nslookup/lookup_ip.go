package nslookup

import (
	"fmt"
	"net"
)

type LookupIPService struct{}

func NewLookupIPService() *LookupIPService {
	return new(LookupIPService)
}

func (s *LookupIPService) LookupIP(address string) (ips []net.IP, err error) {
	if ips, err = net.LookupIP(address); err != nil {
		return ips, fmt.Errorf("LookupIP: %w", err)
	}

	return ips, nil
}
