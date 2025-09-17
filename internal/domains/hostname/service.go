package hostname

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actcmd"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"
	"github.com/samber/lo"
)

type (
	IShellService interface {
		Exec(command shell.ICommand) (err error)
		ExecOutput(command shell.ICommand) (output []byte, err error)
	}

	IActivityService interface {
		ExecuteActivity(ctx context.Context, transaction *activity.Transaction, activityType, name string, payload any, options ...activity.ExecActivityOption) (err error)
	}
)

type Service struct {
	shellService     IShellService
	activityService  IActivityService
	cliExtExecutable string
}

func NewService(shellService IShellService, activityService IActivityService, cliExtExecutable string) *Service {
	return &Service{
		shellService:     shellService,
		activityService:  activityService,
		cliExtExecutable: cliExtExecutable,
	}
}

// UpdateHostname updates device hostname.
func (s *Service) UpdateHostname(hostname string) (err error) {
	if lo.IsEmpty(strings.TrimSpace(hostname)) {
		return fmt.Errorf("UpdateHostname: hostname is empty")
	}

	hostnameCmd := fmt.Sprintf("%s hostname set %s", s.cliExtExecutable, hostname)
	if err = s.shellService.Exec(commands.NewCustomCmd(hostnameCmd)); err != nil {
		return fmt.Errorf("UpdateHostname: %w", err)
	}

	return nil
}

// UpdateHostnameWithTx updates device hostname (using activity transaction).
func (s *Service) UpdateHostnameWithTx(ctx context.Context, tx *activity.Transaction, hostname string) (err error) {
	prevHostname, err := s.GetHostname()
	if err != nil {
		return fmt.Errorf("UpdateHostnameWithTx: %w", err)
	}

	cmd := fmt.Sprintf("%s hostname set %s", s.cliExtExecutable, hostname)
	rlCmd := fmt.Sprintf("%s hostname set %s", s.cliExtExecutable, prevHostname)
	if err = s.activityService.ExecuteActivity(ctx, tx, actcmd.ActivityExecCommand, "update hostname",
		actcmd.NewExecCommandPayload(cmd, rlCmd),
	); err != nil {
		return fmt.Errorf("UpdateHostnameWithTx: %w", err)
	}

	return nil
}

// GetHostname returns current device's hostname.
func (s *Service) GetHostname() (hostname string, err error) {
	output, err := s.shellService.ExecOutput(commands.NewGetHostnameCmd())
	if err != nil {
		return hostname, fmt.Errorf("GetHostname: %w", err)
	}

	return string(output), nil
}
