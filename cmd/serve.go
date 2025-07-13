package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/server"
	"github.com/ananthakumaran/paisa/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var port int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "serve the WEB UI",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := utils.OpenDB()
		model.AutoMigrate(db)

		if os.Getenv("PAISA_DEBUG") == "true" {
			db = db.Debug()
		}

		if err != nil {
			log.Fatal(err)
		}

		// Set up signal handling for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle shutdown signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			log.Infof("Received signal %v, initiating graceful shutdown...", sig)
			cancel()
		}()

		// Start the server with context
		if err := server.ListenWithContext(ctx, db, port); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&port, "port", "p", 7500, "port to listen on")
}
