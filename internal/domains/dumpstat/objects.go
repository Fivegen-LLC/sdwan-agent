package dumpstat

type LinuxInterface struct {
	State    string               `json:"operstate"`
	MTU      int                  `json:"mtu"`
	AddrInfo []LinuxInterfaceAddr `json:"addr_info"` //nolint:tagliatelle // linux api
}

type LinuxInterfaceAddr struct {
	Addr   string `json:"local"`
	Prefix int    `json:"prefixlen"`
}

type LinuxInterfaces []LinuxInterface

type counter int

func newCounter() *counter {
	c := counter(1)
	return &c
}

func (c *counter) get() int {
	num := *c
	*c = num + 1
	return int(num)
}
