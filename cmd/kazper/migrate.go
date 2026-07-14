package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vinzenzs/kazper/internal/config"
	"github.com/vinzenzs/kazper/internal/store"
)

func migrateCmd() *cobra.Command {
	var forceVersion int
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations and exit",
		Long: `Apply pending database migrations and exit.

If a prior migration failed partway, the schema is left in a DIRTY state and
every run (including MIGRATE_ON_START at serve boot) reports it. Recover with:

    kazper migrate --force <version>

which pins the recorded version to <version> WITHOUT running any migration — set
it to the last successfully-applied migration, then re-run "kazper migrate".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(nil)
			if err != nil {
				return err
			}
			if err := cfg.ValidateForMigrate(); err != nil {
				return err
			}
			if cmd.Flags().Changed("force") {
				if forceVersion < 0 {
					return fmt.Errorf("--force requires a non-negative version (the last successfully-applied migration)")
				}
				if err := store.Force(cfg.DatabaseURL, forceVersion); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "forced migration version to %d (dirty flag cleared); run `kazper migrate` to apply pending migrations\n", forceVersion)
				return nil
			}
			if err := store.Migrate(cfg.DatabaseURL); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "migrations applied")
			return nil
		},
	}
	// -1 sentinel: a bare `--force` (no value) is rejected by Cobra before RunE
	// ("flag needs an argument"), satisfying the "no bare force" contract.
	cmd.Flags().IntVar(&forceVersion, "force", -1, "clear a dirty migration state by pinning the schema to <version> without running migrations")
	return cmd
}
