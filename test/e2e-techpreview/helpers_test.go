package e2e_techpreview_test

import (
	_ "embed"
	"testing"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/openshift/machine-config-operator/test/helpers"
)

// `createMCForTest` creates a MachineConfig to be used in an e2e test. The resulting MC will target
// the desired MCP and will be a valid if the input `isValidMC` value is true, or invalid otherwise.
func createMCForTest(mcpName string, isValidMC bool) *mcfgv1.MachineConfig {
	mcName := "99-worker-valid-testfile"
	fileName := "/home/core/test"
	if !isValidMC {
		mcName = "99-worker-invalid-testfile"
		fileName = "/home/core"
	}
	mc := helpers.CreateMC(mcName, mcpName)

	ignConfig := ctrlcommon.NewIgnConfig()
	ignFile := helpers.CreateUncompressedIgn3File(fileName, "data:,hello-world", 420)
	ignConfig.Storage.Files = append(ignConfig.Storage.Files, ignFile)
	rawIgnConfig := helpers.MarshalOrDie(ignConfig)
	mc.Spec.Config.Raw = rawIgnConfig

	return mc
}

func applyMC(t *testing.T, cs *framework.ClientSet, mc *mcfgv1.MachineConfig) func() {
	cleanupFunc := helpers.ApplyMC(t, cs, mc)
	t.Logf("Created new MachineConfig %q", mc.Name)

	return cleanupFunc
}
