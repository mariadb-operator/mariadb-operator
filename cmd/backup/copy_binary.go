package backup

import (
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/spf13/cobra"
)

var copyBinaryCommandTarget string

func init() {
	copyBinaryCommand.Flags().StringVar(&copyBinaryCommandTarget, "copy-binary-to", "",
		"Path to copy the operator binary to.")
	if err := copyBinaryCommand.MarkFlagRequired("copy-binary-to"); err != nil {
		fmt.Printf("error marking 'copy-binary-to' flag as required: %v", err)
		os.Exit(1)
	}
	RootCmd.AddCommand(copyBinaryCommand)
}

var copyBinaryCommand = &cobra.Command{
	Use:   "copy-binary",
	Short: "Copy the operator binary.",
	Long:  "Copies the operator binary into a shared volume for streaming backup or restore Jobs.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}

		logger.Info("copying operator binary", "dest", copyBinaryCommandTarget)
		if err := copyOperatorBinary(copyBinaryCommandTarget); err != nil {
			logger.Error(err, "error copying operator binary", "dest", copyBinaryCommandTarget)
			os.Exit(1)
		}
	},
}
