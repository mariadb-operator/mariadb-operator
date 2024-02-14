package log

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func SetupLogger(level, timeEncoder string, development bool) {
	var lvl zapcore.Level
	var enc zapcore.TimeEncoder

	lvlErr := lvl.UnmarshalText([]byte(level))
	if lvlErr != nil {
		setupLog.Error(lvlErr, "error unmarshalling log level")
		os.Exit(1)
	}
	encErr := enc.UnmarshalText([]byte(timeEncoder))
	if encErr != nil {
		setupLog.Error(encErr, "error unmarshalling time encoder")
		os.Exit(1)
	}
	opts := zap.Options{
		Level:       lvl,
		TimeEncoder: enc,
		Development: development,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
}

func SetupLoggerWithCommand(cmd *cobra.Command) error {
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return fmt.Errorf("error getting 'log-level' flag: %v\n", err)
	}
	logTimeEncoder, err := cmd.Flags().GetString("log-time-encoder")
	if err != nil {
		return fmt.Errorf("error getting 'log-time-encoder' flag: %v\n", err)
	}
	logDev, err := cmd.Flags().GetBool("log-dev")
	if err != nil {
		return fmt.Errorf("error getting 'log-dev' flag: %v\n", err)
	}
	SetupLogger(logLevel, logTimeEncoder, logDev)
	return nil
}
