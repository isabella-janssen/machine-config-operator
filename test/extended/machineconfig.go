package extended

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/machine-config-operator/test/extended/util"
	logger "github.com/openshift/machine-config-operator/test/extended/util/logext"

	"k8s.io/apimachinery/pkg/util/wait"
)

// MachineConfigList handles list of nodes
type MachineConfigList struct {
	ResourceList
}

// MachineConfig struct is used to handle MachineConfig resources in OCP
type MachineConfig struct {
	Resource
	Template
	pool           string
	parameters     []string
	skipWaitForMcp bool
}

// NewMachineConfig create a NewMachineConfig struct
func NewMachineConfig(oc *exutil.CLI, name, pool string) *MachineConfig {
	mc := &MachineConfig{Resource: *NewResource(oc, "mc", name), pool: pool}
	return mc.SetTemplate(*NewMCOTemplate(oc, GenericMCTemplate))
}

// SetTemplate sets the template that will be used by the "create" method in order to create the MC
func (mc *MachineConfig) SetTemplate(template Template) *MachineConfig {
	mc.Template = template
	return mc
}

func (mc *MachineConfig) create() {
	mc.name = mc.name + "-" + exutil.GetRandomString()
	params := []string{"-p", "NAME=" + mc.name, "POOL=" + mc.pool}
	params = append(params, mc.parameters...)
	mc.Create(params...)

	pollerr := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 1*time.Minute, false, func(_ context.Context) (bool, error) {
		stdout, err := mc.Get(`{.metadata.name}`)
		if err != nil {
			logger.Errorf("the err:%v, and try next round", err)
			return false, nil
		}
		if strings.Contains(stdout, mc.name) {
			logger.Infof("mc %s is created successfully", mc.name)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(pollerr, fmt.Sprintf("create machine config %v failed", mc.name))

	if !mc.skipWaitForMcp {
		mcp := NewMachineConfigPool(mc.oc, mc.pool)
		if mc.GetKernelTypeSafe() != "" {
			mcp.SetWaitingTimeForKernelChange() // Since we configure a different kernel we wait longer for completion
		}

		if mc.HasExtensionsSafe() {
			mcp.SetWaitingTimeForExtensionsChange() // Since we configure extra extension we need to wait longer for completion
		}
		mcp.waitForComplete()
	}
}

// DeleteWithWait deletes the MachineConfig and waits for the MCP to be updated
func (mc *MachineConfig) DeleteWithWait() {
	// This method waits a minimum of 1 minute for the MCP to be updated after the MC has been deleted.
	// It is very expensive, since this method is deferred very often and in those cases the MC has been already deleted.
	// In order to improve the performance we do nothing if the MC does not exist.
	if !mc.Exists() {
		logger.Infof("MachineConfig %s does not exist. We will not try to delete it.", mc.GetName())
		return
	}

	mcp := NewMachineConfigPool(mc.oc, mc.pool)
	if mc.GetKernelTypeSafe() != "" {
		mcp.SetWaitingTimeForKernelChange() // If the MC is configuring a different kernel, we increase the waiting period
	}

	err := mc.oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mc.name, "--ignore-not-found=true").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	mcp.waitForComplete()
}

// GetKernelTypeSafe Get the kernelType configured in this MC. If any arror happens it returns an empty string
func (mc *MachineConfig) GetKernelTypeSafe() string {
	return mc.GetSafe(`{.spec.kernelType}`, "")
}

// HasExtensionsSafe  returns true if the MC has any extension configured
func (mc *MachineConfig) HasExtensionsSafe() bool {
	ext := mc.GetSafe(`{.spec.extensions}`, "[]")
	return ext != "[]" && ext != ""
}

// GetAuthorizedKeysByUserAsList returns a list of all authorized keys for the given user
func (mc *MachineConfig) GetAuthorizedKeysByUserAsList(user string) ([]string, error) {
	listKeys := []string{}

	keys, err := mc.Get(fmt.Sprintf(`{.spec.config.passwd.users[?(@.name=="%s")].sshAuthorizedKeys}`, user))
	if err != nil {
		return nil, err
	}

	if keys == "" {
		return listKeys, nil
	}

	jKeys := JSON(keys)
	for _, key := range jKeys.Items() {
		listKeys = append(listKeys, key.ToString())
	}

	return listKeys, err
}
