package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/logger"
	"github.com/rs/zerolog/log"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/Fivegen-LLC/sdwan-agent/infrastructure"
	"github.com/Fivegen-LLC/sdwan-agent/internal/constants"
	"github.com/Fivegen-LLC/sdwan-agent/internal/environment"
)

var (
	env            environment.Environment
	serviceVersion = "0.0.1"
)

func init() {
	var err error
	if env, err = environment.New(); err != nil {
		log.Fatal().Err(err).Msg("error loading environment")
	}
}

func main() {
	logWriter, err := setupRollingLogFile(env.Agent.LogfilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("main")
	}

	log.Output(logWriter)
	if err = logger.SetLogLevel(env.Agent.LogLevel); err != nil {
		log.Fatal().Err(err).Msg("main")
	}

	log.Info().
		Any("app", env).
		Str("agent version", serviceVersion).
		Str("log path", env.Agent.LogfilePath).
		Str("log level", env.Agent.LogLevel).
		Str("device type", env.DeviceType).
		Msg("main: app started")

	cancelCtx, cancelFunc := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	kernel, err := infrastructure.Inject(env)
	if err != nil {
		log.Fatal().Err(err).Msg("main")
	}

	log.Info().Msg("main: start initializing app services...")
	if err = initServices(cancelCtx, kernel); err != nil {
		log.Fatal().Err(err).Msg("main")
	}
	log.Info().Msg("main: app services initialized")

	<-cancelCtx.Done()

	log.Info().Msg("main: stopping app...")
	shutdownServices(kernel)
	log.Info().Msg("main: app gracefully stopped")
}

func initServices(ctx context.Context, kernel *infrastructure.Kernel) (err error) {
	// init publisher
	kernel.InjectWebsocketService().SetRoutes(getWebsocketRoutes(kernel))

	// connect to message broker
	log.Info().Msg("initServices: connecting to MQ broker...")
	mqService := kernel.InjectMQService()
	mqService.RegisterHandlers(getMQRoutes(kernel))
	if err = mqService.Connect(); err != nil {
		return fmt.Errorf("initServices: connection to message broker failed")
	}
	log.Info().Msg("initServices: connected to MQ broker")

	// try to rollback dangling transactions
	log.Info().Msg("initServices: trying to rollback dangling transactions...")
	if err = kernel.InjectActivityService().TryRollbackDanglingTransactions(ctx, 1); err != nil {
		log.Error().Err(err).Msg("initServices: rollback dangling transactions error")
	} else {
		log.Info().Msg("initServices: dangling transactions successfully closed")
	}

	// reload cache after updating configs via transactions (sync with new configuration)
	log.Info().Msg("initServices: reload data cache")
	if err = kernel.InjectConfigService().ReloadCache(); err != nil {
		return fmt.Errorf("initServices: reload data cache failed")
	}

	// sync hosts
	log.Info().Msg("initServices: sync hosts")
	if err = kernel.InjectNSLookupService().SyncHosts(); err != nil {
		log.Error().Err(err).Msg("initServices: sync hosts error")
	}

	// activate always available handlers
	if err = mqService.ActivateHandler(constants.MQAgentGetConfig); err != nil {
		return fmt.Errorf("initServices: %w", err)
	}

	if err = mqService.ActivateHandler(constants.MQAgentHubListPorts); err != nil {
		return fmt.Errorf("initServices: %w", err)
	}

	if err = mqService.ActivateHandler(constants.MQAgentDebugDumpHeap); err != nil {
		return fmt.Errorf("initServices: %w", err)
	}

	go kernel.InjectCommandBufferService().Start(ctx)

	log.Info().Msg("initServices: starting monitoring service...")
	go kernel.InjectPonyService().Start(ctx)
	go kernel.InjectPonyEventService().StartListenEvents(ctx)
	log.Info().Msg("initServices: monitoring service started")

	// start agent main logic controller
	log.Info().Msg("initServices: starting app state controller...")
	kernel.BuildAppStateService()
	go kernel.InjectAppStateService().Run(ctx)
	log.Info().Msg("initServices: app state controller started")

	log.Info().Msg("initServices: starting discovery service...")
	go kernel.InjectDiscoveryMonitoringService().Start(ctx)
	log.Info().Msg("initServices: discovery service started")

	return nil
}

func shutdownServices(kernel *infrastructure.Kernel) {
	if err := kernel.InjectWebsocketService().Stop(); err != nil {
		log.Error().Err(err).Msg("shutdownServices: websocket service shutdown error")
	}

	if err := kernel.InjectMQService().Close(); err != nil {
		log.Error().Err(err).Msg("shutdownServices: close MQ error")
	}

	if err := kernel.DB.Close(); err != nil {
		log.Error().Err(err).Msg("shutdownServices: close badger error")
	}
}

func setupRollingLogFile(filename string) (logWriter *lumberjack.Logger, err error) {
	// create log dir if not exists
	if err = os.MkdirAll(filepath.Dir(filename), constants.LogFilePerm); err != nil {
		return logWriter, fmt.Errorf("setupRollingLogFile: %w", err)
	}

	if _, statErr := os.Stat(filename); statErr != nil {
		if !os.IsNotExist(statErr) {
			return logWriter, fmt.Errorf("setupRollingLogFile: %w", statErr)
		}

		// create new log file
		logFile, err := os.OpenFile(filename, os.O_CREATE, constants.LogFilePerm)
		if err != nil {
			return logWriter, fmt.Errorf("setupRollingLogFile: %w", err)
		}
		defer logFile.Close()
	}

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    15,   // 100 megabytes per log file
		MaxAge:     30,   // store retained log files for 30 days
		MaxBackups: 10,   // store maximum 10 retained log files
		Compress:   true, // compress files wia gzip
	}, nil
}
