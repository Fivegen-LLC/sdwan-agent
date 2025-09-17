package firstport

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/activity/handlers/actfile"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/rollback"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell"
	"github.com/Fivegen-LLC/sdwan-lib/pkg/shell/commands"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
)

const (
	firstPortConfig = "port1"
)

type (
	IShellService interface {
		Exec(command shell.ICommand) (err error)
	}

	IActivityService interface {
		ExecuteActivity(ctx context.Context, transaction *activity.Transaction, activityType, name string, payload any, options ...activity.ExecActivityOption) (err error)
	}

	Service struct {
		shellService    IShellService
		activityService IActivityService
	}
)

func NewService(shellService IShellService, activityService IActivityService) *Service {
	return &Service{
		shellService:    shellService,
		activityService: activityService,
	}
}

// ClearStatic clears static configuration from first port for services usage (LAN mode).
func (s *Service) ClearStatic() (rollbacks rollback.Rollbacks, err error) {
	rollbacks = rollback.New()
	fullPath := path.Join(constants.NetworkInterfacesPath, firstPortConfig)
	portConfigStat, err := os.Stat(fullPath)
	if err != nil {
		return rollbacks, fmt.Errorf("ClearStatic: %w", err)
	}

	backupPortConfig, err := os.ReadFile(fullPath)
	if err != nil {
		return rollbacks, fmt.Errorf("ClearStatic: %w", err)
	}

	newConfig := []byte(`auto port1
allow-hotplug port1
iface port1 inet manual
`)

	if err = os.WriteFile(fullPath, newConfig, portConfigStat.Mode().Perm()); err != nil {
		return rollbacks, fmt.Errorf("ClearStatic: %w", err)
	}
	rollbacks = append(rollbacks, func() (err error) {
		if err = os.WriteFile(fullPath, backupPortConfig, portConfigStat.Mode().Perm()); err != nil {
			return fmt.Errorf("rollback error: %w", err)
		}
		if err = s.shellService.Exec(commands.NewIPAddrFlush(firstPortConfig)); err != nil {
			return fmt.Errorf("rollback error: %w", err)
		}
		return nil
	})

	// flush port (reload interface)
	if err = s.shellService.Exec(commands.NewIPAddrFlush(firstPortConfig)); err != nil {
		return rollbacks, fmt.Errorf("ClearStatic: %w", err)
	}

	return rollbacks, nil
}

// SetupStatic installs static configuration for first port (for ZTP).
func (s *Service) SetupStatic() (rollbacks rollback.Rollbacks, err error) {
	rollbacks = rollback.New()
	fullPath := path.Join(constants.NetworkInterfacesPath, firstPortConfig)
	portConfigStat, err := os.Stat(fullPath)
	if err != nil {
		return rollbacks, fmt.Errorf("SetupStatic: %w", err)
	}

	backupPortConfig, err := os.ReadFile(fullPath)
	if err != nil {
		return rollbacks, fmt.Errorf("SetupStatic: %w", err)
	}

	newConfig := []byte(`allow-hotplug port1
iface port1 inet static
	address 192.168.1.1
	netmask 255.255.255.0
`)

	if err = os.WriteFile(fullPath, newConfig, portConfigStat.Mode().Perm()); err != nil {
		return rollbacks, fmt.Errorf("SetupStatic: %w", err)
	}
	rollbacks = append(rollbacks, func() (err error) {
		if err = os.WriteFile(fullPath, backupPortConfig, portConfigStat.Mode().Perm()); err != nil {
			return fmt.Errorf("rollback error: %w", err)
		}
		return nil
	})

	return rollbacks, nil
}

// SetupStaticWithTx installs static configuration for first port (for ZTP) using activity transaction.
func (s *Service) SetupStaticWithTx(ctx context.Context, tx *activity.Transaction) (err error) {
	fullPath := path.Join(constants.NetworkInterfacesPath, firstPortConfig)
	backupPortConfig, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("SetupStaticWithTx: %w", err)
	}

	newConfig := []byte(`allow-hotplug port1
iface port1 inet static
	address 192.168.1.1
	netmask 255.255.255.0
`)

	if err = s.activityService.ExecuteActivity(ctx, tx, actfile.ActivityUpdateFile, "update first port file",
		actfile.NewUpdateFilePayload(fullPath, constants.FilePerm, newConfig, backupPortConfig),
	); err != nil {
		return fmt.Errorf("SetupStaticWithTx: %w", err)
	}

	return nil
}
