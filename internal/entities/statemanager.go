package entities

type (
	ActionType string

	StateItem struct {
		State   string
		Message string
	}

	Event struct {
		Action  ActionType
		Message string
	}
)

var (
	StartingState    = "Starting"
	WorkingState     = "Working"
	FailedState      = "Failed"
	ApplyConfigState = "ApplyConfig"
	ResetState       = "Reset"
	MaintenanceState = "Maintenance"

	ToStartAction       ActionType = "ToStart"
	ToWorkAction        ActionType = "ToWork"
	ToFailAction        ActionType = "ToFail"
	ToApplyConfigAction ActionType = "ToApplyConfig"
	ToResetAction       ActionType = "ToReset"
	ToMaintenanceAction ActionType = "ToMaintenance"
)
