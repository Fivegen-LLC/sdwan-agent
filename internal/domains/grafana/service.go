package grafana

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog/log"

	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
)

const (
	templateName          = "grafanaTemplates"
	grafanaServiceName    = "grafana-agent-flow"
	grafanaConfigTemplate = "grafana_config.tpl"
)

//go:embed templates/*
var templatesFS embed.FS

type Service struct {
	grafanaConfigPath string
	mimirPort         int
	lokiPort          int

	templates *template.Template
}

func NewService(grafanaConfigPath string, mimirPort, lokiPort int) *Service {
	templates, err := template.New(templateName).
		ParseFS(templatesFS, "templates/*tpl")
	if err != nil {
		log.Fatal().Err(err).Msg("NewService")
	}

	return &Service{
		grafanaConfigPath: grafanaConfigPath,
		mimirPort:         mimirPort,
		lokiPort:          lokiPort,

		templates: templates,
	}
}

// ConfigureAgent configures grafana agent if settings changed.
func (s *Service) ConfigureAgent(serverAddr string) (err error) {
	configDir := filepath.Dir(s.grafanaConfigPath)

	// create directory if not exists
	if _, dirErr := os.Stat(configDir); dirErr != nil {
		if !os.IsNotExist(dirErr) {
			return fmt.Errorf("ConfigureAgent: %w", dirErr)
		}

		if err = os.MkdirAll(configDir, os.ModePerm); err != nil {
			return fmt.Errorf("ConfigureAgent: %w", err)
		}
	}

	// write grafana config
	var buffer bytes.Buffer
	data := struct {
		MIMIR string
		LOKI  string
	}{
		MIMIR: fmt.Sprintf("%s:%d/api/v1/push", serverAddr, s.mimirPort),
		LOKI:  fmt.Sprintf("%s:%d/loki/api/v1/push", serverAddr, s.lokiPort),
	}
	if err = s.templates.ExecuteTemplate(&buffer, grafanaConfigTemplate, data); err != nil {
		return fmt.Errorf("ConfigureAgent: %w", err)
	}

	stat, fileErr := os.Stat(s.grafanaConfigPath)
	if fileErr != nil {
		if !os.IsNotExist(fileErr) {
			return fmt.Errorf("ConfigureAgent: %w", fileErr)
		}

		if err = os.WriteFile(s.grafanaConfigPath, buffer.Bytes(), constants.FilePerm); err != nil {
			return fmt.Errorf("ConfigureAgent: %w", err)
		}

		if err = s.restartGrafanaService(); err != nil {
			return fmt.Errorf("ConfigureAgent: %w", err)
		}

		return nil
	}

	// compare diff
	oldContent, err := os.ReadFile(s.grafanaConfigPath)
	if err != nil {
		return fmt.Errorf("ConfigureAgent: %w", err)
	}

	if string(oldContent) == buffer.String() {
		return nil
	}

	// overwrite file
	if err = os.WriteFile(s.grafanaConfigPath, buffer.Bytes(), stat.Mode().Perm()); err != nil {
		return fmt.Errorf("ConfigureAgent: %w", fileErr)
	}

	if err = s.restartGrafanaService(); err != nil {
		return fmt.Errorf("ConfigureAgent: %w", err)
	}

	return nil
}

func (s *Service) restartGrafanaService() (err error) {
	reloadCmd := exec.Command("systemctl", "daemon-reload")
	if err = reloadCmd.Run(); err != nil {
		return fmt.Errorf("restartGrafanaService: %w", err)
	}

	restartCmd := exec.Command("systemctl", "restart", grafanaServiceName)
	if err = restartCmd.Run(); err != nil {
		return fmt.Errorf("restartGrafanaService: %w", err)
	}

	return nil
}
