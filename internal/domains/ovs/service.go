package ovs

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

type Service struct{}

func NewService() *Service {
	return new(Service)
}

func (s *Service) SetupOVSManager(ofControllerAddr string) (err error) {
	// example: ovs-vsctl set-manager ptcp:6640
	listenPort := fmt.Sprintf("ptcp:%s", strings.Split(ofControllerAddr, ":")[1])
	setManagerCmd := exec.Command("ovs-vsctl", "set-manager", listenPort)
	log.Debug().Msgf("Executing cmd: %s", setManagerCmd.String())

	output, err := setManagerCmd.Output()
	if err != nil {
		return fmt.Errorf("SetupOVSManager: %w: output: %s", err, string(output))
	}

	log.Debug().Msg("Manager successfully set!")
	return nil
}
