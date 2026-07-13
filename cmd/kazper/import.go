package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/vinzenzs/kazper/internal/config"
	"github.com/vinzenzs/kazper/internal/dataexport"
	"github.com/vinzenzs/kazper/internal/store"
)

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import a JSON export into an empty database",
		Long: `Import restores a JSON export produced by "kazper export" into the
configured database. It refuses unless the database is empty and at the
export's exact migration head, runs in a single transaction inserting
parents before children, and verifies row counts against the manifest —
rolling back entirely on any error. Pass - to read the export from stdin.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("import requires exactly one argument: the export file (use - for stdin)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(nil)
			if err != nil {
				return err
			}
			if err := cfg.ValidateForMigrate(); err != nil {
				return err
			}

			var data []byte
			if args[0] == "-" {
				data, err = io.ReadAll(cmd.InOrStdin())
			} else {
				data, err = os.ReadFile(args[0])
			}
			if err != nil {
				return fmt.Errorf("read export: %w", err)
			}

			ctx := cmd.Context()
			pool, err := store.NewPool(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			summary, err := dataexport.Import(ctx, pool, data)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "import complete: %d rows across %d tables\n", summary.Rows, summary.Tables)
			return nil
		},
	}
}
