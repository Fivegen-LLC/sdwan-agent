package entities

type Action string

func (s Action) String() string {
	return string(s)
}

var (
	Reset    Action = "reset"
	Reboot   Action = "reboot"
	PowerOff Action = "poweroff"
)

type ExecDeviceAction struct {
	Action Action `json:"action" validate:"required"`
}
