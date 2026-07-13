package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/vinzenzs/kazper/internal/config"
	"github.com/vinzenzs/kazper/internal/dataexport"
	"github.com/vinzenzs/kazper/internal/store"
)

func exportCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all user data as a single JSON document",
		Long: `Export writes a full logical JSON export of the database (all user-data
tables plus a manifest) to stdout, or to a file with --out. It connects
directly to DATABASE_URL like migrate. Diagnostics go to stderr so stdout
stays pipeable. See BACKUP.md for how this differs from the pg_dump backup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(nil)
			if err != nil {
				return err
			}
			if err := cfg.ValidateForMigrate(); err != nil {
				return err
			}

			ctx := cmd.Context()
			pool, err := store.NewPool(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			w := cmd.OutOrStdout()
			if outPath != "" {
				f, err := os.Create(outPath)
				if err != nil {
					return fmt.Errorf("create %s: %w", outPath, err)
				}
				defer f.Close()
				w = io.Writer(f)
			}

			if err := dataexport.Export(ctx, pool, version, time.Now(), w); err != nil {
				return err
			}
			if outPath != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "export written to %s\n", outPath)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "export complete")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "write the export to a file instead of stdout")
	return cmd
}
