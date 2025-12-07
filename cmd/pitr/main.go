package pitr

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log

	s3           bool
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	s3TLS        bool
	s3CACertPath string
	s3Prefix     string
)

var RootCmd = &cobra.Command{
	Use:   "pitr",
	Short: "PITR.",
	Long:  `Pulls binary logs from object storage and restores them to a given point-in-time.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting ppoint-in-time recovery")

		_, cancel := newContext()
		defer cancel()
	},
}

func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
}
