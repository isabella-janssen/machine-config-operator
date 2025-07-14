package e2e_techpreview_test

import (
	_ "embed"
	"fmt"
	"testing"

	ign3types "github.com/coreos/ignition/v2/config/v3_5/types"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/openshift/machine-config-operator/test/helpers"
)

var skipCleanupAlways bool
var skipCleanupOnlyAfterFailure bool

func newMachineConfig(name, pool string) *mcfgv1.MachineConfig {
	mode := 420
	testfiledata := fmt.Sprintf("data:,%s-%s", name, pool)
	path := fmt.Sprintf("/etc/%s-%s", name, pool)
	file := ign3types.File{
		Node: ign3types.Node{
			Path: path,
		},
		FileEmbedded1: ign3types.FileEmbedded1{
			Contents: ign3types.Resource{
				Source: &testfiledata,
			},
			Mode: &mode,
		},
	}

	return helpers.NewMachineConfig(name, helpers.MCLabelForRole(pool), "", []ign3types.File{file})
}

// Registers a cleanup function, making it idempotent, and wiring up the skip
// cleanup checks which will cause cleanup to be skipped under certain
// conditions.
func makeIdempotentAndRegister(t *testing.T, cleanupFunc func()) func() {
	cfg := helpers.IdempotentConfig{
		SkipAlways:        skipCleanupAlways,
		SkipOnlyOnFailure: skipCleanupOnlyAfterFailure,
	}

	return helpers.MakeConfigurableIdempotentAndRegister(t, cfg, cleanupFunc)
}

// newMachineConfigWithExtensions returns the same base MC, but adds the given extensions to trigger an image rebuild
func newMachineConfigTriggersImageRebuild(name, pool string, exts []string) *mcfgv1.MachineConfig {
	mc := newMachineConfig(name, pool)
	mc.Spec.Extensions = append(mc.Spec.Extensions, exts...)
	return mc
}

func applyMC(t *testing.T, cs *framework.ClientSet, mc *mcfgv1.MachineConfig) func() {
	cleanupFunc := helpers.ApplyMC(t, cs, mc)
	t.Logf("Created new MachineConfig %q", mc.Name)

	return makeIdempotentAndRegister(t, func() {
		cleanupFunc()
		t.Logf("Deleted MachineConfig %q", mc.Name)
	})
}
