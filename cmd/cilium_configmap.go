package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"k8s-diagnostic/internal/diagnostic/system"

	"github.com/spf13/cobra"
)

var (
	cfgJSON bool
	cfgKey  string
)

// ciliumConfigmapCmd prints the cilium-config ConfigMap
var ciliumConfigmapCmd = &cobra.Command{
	Use:   "configmap",
	Short: "Inspect cilium-config ConfigMap",
	Run: func(cmd *cobra.Command, args []string) {
		// Respect global kubeconfig by exporting env var for kubectl
		kubeconfig, _ := rootCmd.Flags().GetString("kubeconfig")
		if kubeconfig != "" {
			os.Setenv("KUBECONFIG", kubeconfig)
		}

		cm, err := system.GetConfigMapData("kube-system", "cilium-config")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}

		if cfgKey != "" {
			if cm.Data == nil {
				fmt.Printf("ERROR: cilium-config has no data\n")
				os.Exit(1)
			}
			if val, ok := cm.Data[cfgKey]; ok {
				fmt.Printf("%s\n", val)
				return
			}
			fmt.Printf("ERROR: key '%s' not found\n", cfgKey)
			os.Exit(1)
		}

		if cfgJSON {
			// Print only the data section to focus on user-tunable keys
			if cm.Data == nil {
				fmt.Printf("{}\n")
				return
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(cm.Data)
			return
		}

		// Default pretty print (key=value)
		if cm.Data == nil {
			fmt.Println("(empty)")
			return
		}
		for k, v := range cm.Data {
			fmt.Printf("%s=%s\n", k, v)
		}
	},
}

func init() {
	ciliumCmd.AddCommand(ciliumConfigmapCmd)
	ciliumConfigmapCmd.Flags().BoolVar(&cfgJSON, "json", false, "print ConfigMap data as pretty JSON")
	ciliumConfigmapCmd.Flags().StringVar(&cfgKey, "key", "", "print value of a specific key")
}
