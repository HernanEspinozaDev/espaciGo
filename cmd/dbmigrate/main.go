package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func main() {
	migrationDir := flag.String("dir", "db/migrations", "directory containing V<version>__<description>.sql files")
	planOnly := flag.Bool("plan-only", false, "print the read-only applied/pending migration inventory without executing SQL")
	flag.Parse()

	settings, err := loadConnectionSettings(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
	defer cancel()
	if *planOnly {
		var plan migrator.PlanResult
		if settings.mode == "cloudsql" {
			databaseURL, password, urlErr := settings.databaseURLAndPasswordFile()
			if urlErr != nil {
				fmt.Fprintln(os.Stderr, urlErr)
				os.Exit(2)
			}
			plan, err = migrator.PlanCloudSQL(ctx, databaseURL, password, *migrationDir, settings.instance, settings.privateIP, settings.setRole)
		} else {
			plan, err = migrator.Plan(ctx, settings.databaseURL, *migrationDir, settings.setRole)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "migration plan failed: %v\n", err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			fmt.Fprintln(os.Stderr, "could not encode migration plan")
			os.Exit(1)
		}
		return
	}
	var result migrator.Result
	if settings.mode == "cloudsql" {
		databaseURL, password, urlErr := settings.databaseURLAndPasswordFile()
		if urlErr != nil {
			fmt.Fprintln(os.Stderr, urlErr)
			os.Exit(2)
		}
		result, err = migrator.RunCloudSQLWithRole(ctx, databaseURL, password, *migrationDir, settings.instance, settings.privateIP, settings.setRole)
	} else {
		result, err = migrator.RunWithRole(ctx, settings.databaseURL, *migrationDir, settings.setRole)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
	if len(result.Applied) == 0 {
		fmt.Println("sin cambios")
		return
	}
	fmt.Printf("applied migrations: %v\n", result.Applied)
}
