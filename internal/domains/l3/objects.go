package l3

type (
	// NeighborOutput stores output from bgp neighbor --json.
	NeighborOutput struct {
		Conf struct {
			LocalASN      int    `json:"local_asn"`        //nolint: tagliatelle // BGP API
			RemoteAddress string `json:"neighbor_address"` //nolint: tagliatelle // BGP API, peer address
			RemoteAS      int    `json:"peer_asn"`         //nolint: tagliatelle // BGP API, peer asn
		} `json:"conf"`
		State struct {
			SessionState int `json:"session_state"` //nolint: tagliatelle // BGP API
		} `json:"state"`
		Timers struct {
			Config struct {
				Holdtime  int64 `json:"hold_time"`          //nolint: tagliatelle // BGP API
				Keepalive int64 `json:"keepalive_interval"` //nolint: tagliatelle // BGP API
			}
			State struct {
				Downtime *struct {
					Seconds int64 `json:"seconds"`
				} `json:"downtime"`
				Uptime *struct {
					Seconds int64 `json:"seconds"`
				} `json:"uptime"`
				Holdtime  int64 `json:"negotiated_hold_time"` //nolint: tagliatelle // BGP API
				Keepalive int64 `json:"keepalive_interval"`   //nolint: tagliatelle // BGP API
			} `json:"state"`
		} `json:"timers"`
		Transport struct {
			LocalAddress string `json:"local_address"` //nolint: tagliatelle // BGP API, link ip (local address)
		} `json:"transport"`
		AfiSafis []struct {
			State struct {
				Advertised int64 `json:"advertised"`
				Received   int64 `json:"received"`
			} `json:"state"`
		} `json:"afi_safis"` //nolint: tagliatelle // BGP API
	}

	NeighborOutputs []NeighborOutput
)

type (
	// NeighborAdvOutput stores output for adv-in and adv-out data
	// adv-in: bgp neighbor 10.101.10.10 adj-in --json
	// adv-out: bgp neighbor 10.101.10.10 adj-out --json.
	NeighborAdvOutput struct {
		NLRI struct {
			Prefix string `json:"prefix"`
		} `json:"nlri"`
		Age   int64 `json:"age"`
		Attrs []struct {
			// type = 2 - as paths
			// type = 3 - nexthop
			Type    int `json:"type"`
			ASPaths []struct {
				ASNs []int `json:"asns"`
			} `json:"as_paths"` //nolint: tagliatelle // BGP API
			Nexthop string `json:"nexthop"`
		} `json:"attrs"`
	}
)

type (
	// NeighborInfo stores result BGP information (sends to server).
	NeighborInfo struct {
		LocalAddr      string              `json:"localAddr"`
		LocalAS        int                 `json:"localAs"`
		PeerAddr       string              `json:"peerAddr"`
		PeerAS         int                 `json:"peerAs"`
		State          string              `json:"state"`
		Holdtime       int64               `json:"holdtime"`
		OperHoldtime   int64               `json:"operHoldtime"`
		OperKeepalive  int64               `json:"operKeepalive"`
		Keepalive      int64               `json:"keepalive"`
		Uptime         *int64              `json:"uptime,omitempty"`
		Downtime       *int64              `json:"downtime,omitempty"`
		AdvIns         []NeighborAdvertise `json:"advIns"`
		AdvOuts        []NeighborAdvertise `json:"advOuts"`
		NeighborOutput string              `json:"neighborOutput"` // output for bgp neighbor 10.101.10.10
	}

	NeighborInfos []NeighborInfo

	NeighborAdvertise struct {
		Network string                    `json:"network"`
		Nexthop string                    `json:"nexthop"`
		Age     int64                     `json:"age"`
		ASPaths []NeighborAdvertiseASPath `json:"asPaths"`
	}

	NeighborAdvertiseASPath struct {
		ASNs []int `json:"asns"`
	}
)
