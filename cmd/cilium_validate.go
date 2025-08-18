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

		fmt.Println("🛡️ Cilium Feature Status Check")

		enabled := 0
		available := 0

		for _, feature := range features {
			if err := system.ValidateFeatureByName(ctx, feature); err != nil {
				displayName := getFeatureDisplayName(feature)
				status := getSimpleStatus(err.Error())
				fmt.Printf("💡 %s - %s\n", displayName, status)
				available++
			} else {
				displayName := getFeatureDisplayName(feature)
				fmt.Printf("✅ %s - Active & Working\n", displayName)
				enabled++
			}
		}

		fmt.Printf("\n📊 Summary: %d features active | %d features available\n", enabled, available)

		if available > 0 {
			fmt.Printf("ℹ️  %d additional features can be enabled if needed\n", available)
		}
	},
}

// getFeatureDisplayName converts feature names to user-friendly display names
func getFeatureDisplayName(featureName string) string {
	displayNames := map[string]string{
		"bgp-control-plane":             "BGP Control Plane",
		"dns-policies":                  "DNS-based Policies",
		"egress-gateway":                "Egress Gateway",
		"gateway-api":                   "Gateway API Support",
		"host-firewall":                 "Host Firewall",
		"ipsec":                         "IPsec Encryption",
		"kube-proxy-replacement-strict": "Kube-proxy Replacement (Strict)",
		"l2-announcements":              "L2 Load Balancer Announcements",
		"nodeport":                      "NodePort Services",
		"wireguard":                     "WireGuard Encryption",
	}

	if displayName, exists := displayNames[featureName]; exists {
		return displayName
	}

	// Fallback: convert kebab-case to Title Case
	parts := strings.Split(featureName, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

// getSimpleStatus converts technical error messages to user-friendly status
func getSimpleStatus(errorMessage string) string {
	errorLower := strings.ToLower(errorMessage)

	switch {
	case strings.Contains(errorLower, "not met"):
		return "Not Enabled"
	case strings.Contains(errorLower, "enable"):
		return "Requires Configuration"
	case strings.Contains(errorLower, "prerequisite"):
		return "Prerequisites Missing"
	default:
		return "Available"
	}
}

func init() {
	ciliumCmd.AddCommand(ciliumValidateCmd)
	ciliumValidateCmd.Flags().BoolVar(&validateAll, "all", false, "validate all known features")
	ciliumValidateCmd.Flags().StringVar(&validateFeatures, "features", "", "comma-separated feature list to validate")
}
