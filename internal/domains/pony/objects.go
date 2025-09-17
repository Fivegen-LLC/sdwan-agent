package pony

type (
	tunnelState struct {
		Tunnel    string `json:"tunnel"`
		IsActive  bool   `json:"isActive"`
		HubSerial string `json:"hubSerial"`
		TableID   int    `json:"tableId"`
	}

	tunnelStates []tunnelState

	cluster struct {
		Network string       `json:"network"`
		States  tunnelStates `json:"states"`
	}

	clusters []cluster
)
