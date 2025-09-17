package entities

type LTEStat struct {
	IMEI          string            `json:"imei"`
	OperatorCode  string            `json:"operatorCode"`
	OperatorName  string            `json:"operatorName"`
	Model         string            `json:"model"`
	PowerState    string            `json:"powerState"`
	State         string            `json:"state"`
	DevicePath    string            `json:"devicePath"`
	Port          string            `json:"port"`
	SignalQuality SignalQualityStat `json:"signalQuality"`
	// general
	Type         string `json:"type"`
	HWAddr       string `json:"hwAddr"`
	MTU          int    `json:"mtu"`
	GeneralState string `json:"generalState"`
	Connection   string `json:"connection"`
	// ip
	IPGateway   string   `json:"ipGateway"`
	IPAddresses []string `json:"ipAddresses"`
	DNS         []string `json:"dns"`
}

type SignalQualityStat struct {
	Recent string `json:"recent"`
	Value  string `json:"value"`
}

type LTEStats []LTEStat
