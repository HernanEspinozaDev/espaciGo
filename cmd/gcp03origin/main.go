package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03origin"
)

func main() {
	config, err := gcp03origin.LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	switch config.Phase {
	case gcp03origin.PhaseBootstrap:
		password, err := gcp03origin.ReadAdminPassword(config.PasswordFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		report, err := gcp03origin.RunBootstrap(ctx, config, password)
		if encodeErr := gcp03origin.WriteJSON(os.Stdout, report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 origin bootstrap report could not be encoded")
			os.Exit(1)
		}
		if err != nil || !report.Passed {
			fmt.Fprintln(os.Stderr, "GCP-03 origin bootstrap stopped; reconcile its outcome before any retry")
			os.Exit(1)
		}
	case gcp03origin.PhaseObserve:
		report, err := gcp03origin.Observe(ctx, config)
		if encodeErr := gcp03origin.WriteJSON(os.Stdout, report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 origin observer report could not be encoded")
			os.Exit(1)
		}
		if err != nil || !report.Passed {
			fmt.Fprintln(os.Stderr, "GCP-03 origin observer found a failed pre-restore check")
			os.Exit(1)
		}
	case gcp03origin.PhaseMembership:
		password, err := gcp03origin.ReadAdminPassword(config.PasswordFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		report, err := gcp03origin.RunMembership(ctx, config, password)
		if encodeErr := gcp03origin.WriteJSON(os.Stdout, report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 origin membership report could not be encoded")
			os.Exit(1)
		}
		if err != nil || !report.Passed {
			fmt.Fprintln(os.Stderr, "GCP-03 origin membership remediation stopped; reconcile its outcome before retry")
			os.Exit(1)
		}
	case gcp03origin.PhaseMembershipCheck:
		password, err := gcp03origin.ReadAdminPassword(config.PasswordFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		report, err := gcp03origin.RunMembershipDiagnostic(ctx, config, password)
		if encodeErr := gcp03origin.WriteJSON(os.Stdout, report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 origin membership diagnostic report could not be encoded")
			os.Exit(1)
		}
		if err != nil || !report.ReportComplete {
			fmt.Fprintln(os.Stderr, "GCP-03 origin membership diagnostic stopped; do not retry until its stage and outcome are reviewed")
			os.Exit(1)
		}
	case gcp03origin.PhaseOwnerACLCheck:
		password, err := gcp03origin.ReadAdminPassword(config.PasswordFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		report, err := gcp03origin.RunOwnerACLDiagnostic(ctx, config, password)
		if encodeErr := gcp03origin.WriteJSON(os.Stdout, report); encodeErr != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 origin owner/ACL diagnostic report could not be encoded")
			os.Exit(1)
		}
		if err != nil || !report.ReportComplete {
			fmt.Fprintln(os.Stderr, "GCP-03 origin owner/ACL diagnostic stopped; review its stage and outcome before retry")
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "GCP-03 origin phase is unsupported")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "GCP-03 origin phase %s completed\n", config.Phase)
}
