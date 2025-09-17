package entities

type HealthcheckStatus struct {
	CompletedSteps []string `json:"completedSteps"`
	ExecFinished   bool     `json:"execFinished"`
	Error          string   `json:"error"`
	ErrorStep      string   `json:"errorStep"`
}
