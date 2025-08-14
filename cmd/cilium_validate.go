package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"k8s-diagnostic/internal/diagnostic/system"

	"github.com/spf13/cobra"
)

var (
	validateAll      bool
	validateFeatures string
)

// ciliumValidateCmd validates Cilium feature prerequisites
var ciliumValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Cilium feature prerequisites",
	Run: func(cmd *cobra.Command, args []string) {
		// Respect global kubeconfig by exporting env var for kubectl
		kubeconfig, _ := rootCmd.Flags().GetString("kubeconfig")
		if kubeconfig != "" {
			os.Setenv("KUBECONFIG", kubeconfig)
		}

		ctx := context.Background()

		var features []string
		if validateAll {
			features = system.ListKnownFeatures()
			sort.Strings(features)
		} else if strings.TrimSpace(validateFeatures) != "" {
			for _, f := range strings.Split(validateFeatures, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					features = append(features, f)
				}
			}
		} else {
			fmt.Println("ERROR: specify --all or --features <list>")
			os.Exit(2)
		}

		if len(features) == 0 {
			fmt.Println("ERROR: no features specified")
			os.Exit(2)
		}

		fmt.Println("Validating Cilium feature prerequisites...")

		failed := 0
		for _, feature := range features {
			if err := system.ValidateFeatureByName(ctx, feature); err != nil {
				fmt.Printf("[%s] FAIL: %s\n", feature, err.Error())
				failed++
			} else {
				fmt.Printf("[%s] OK\n", feature)
			}
		}

		if failed > 0 {
			fmt.Printf("\n%d feature(s) failed validation\n", failed)
			os.Exit(1)
		}
	},
}

func init() {
	ciliumCmd.AddCommand(ciliumValidateCmd)
	ciliumValidateCmd.Flags().BoolVar(&validateAll, "all", false, "validate all known features")
	ciliumValidateCmd.Flags().StringVar(&validateFeatures, "features", "", "comma-separated feature list to validate")
}
