/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package translator

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/volume/util"
)

// ControllerSELinuxTranslator is implementation of SELinuxLabelTranslator that can be used in kube-controller-manager (KCM).
// A real SELinuxLabelTranslator would be able to file empty parts of SELinuxOptions from the operating system defaults (/etc/selinux/*).
// KCM often runs as a container and cannot access /etc/selinux on the host. Even if it could, KCM can run on a different distro
// than the actual worker nodes.
// Therefore do not even try to file the defaults, use only fields filed in the provided SELinuxOptions.
type ControllerSELinuxTranslator struct{}

var _ util.SELinuxLabelTranslator = &ControllerSELinuxTranslator{}

func (c *ControllerSELinuxTranslator) SELinuxEnabled() bool {
	// The controller must have been explicitly enabled, so expect that all nodes have SELinux enabled.
	return true
}

func (c *ControllerSELinuxTranslator) SELinuxOptionsToFileLabel(opts *v1.SELinuxOptions) (string, error) {
	if opts == nil {
		return "", nil
	}
	// kube-controller-manager cannot access SELinux defaults in /etc/selinux on nodes.
	// Just concatenate the existing fields and do not try to default the missing ones.
	parts := []string{
		opts.User,
		opts.Role,
		opts.Type,
		opts.Level,
	}
	label := strings.Join(parts, ":")
	if label == ":::" {
		// Empty SELinuxOptions should have the same behavior as nil
		return "", nil
	}
	return label, nil
}

// Conflicts returns true if two SELinux labels conflict.
// These labels must be generated by SELinuxOptionsToFileLabel above
// (the function expects strict nr. of elements in the labels).
// Since this translator cannot default missing components,
// the missing components are treated as incomparable and they do not
// conflict with anything.
// Example: "system_u:system_r:container_t:s0:c1,c2" *does not* conflict with ":::s0:c1,c2",
// because the node that will run such a Pod may expand "":::s0:c1,c2" to "system_u:system_r:container_t:s0:c1,c2".
// However, "system_u:system_r:container_t:s0:c1,c2" *does* conflict with ":::s0:c98,c99".
func (c *ControllerSELinuxTranslator) Conflicts(labelA, labelB string) bool {
	partsA := strings.SplitN(labelA, ":", 4)
	partsB := strings.SplitN(labelB, ":", 4)

	// Reorder, so partsA is always longer than partsB
	if len(partsA) < len(partsB) {
		partsB, partsA = partsA, partsB
	}

	for len(partsB) < len(partsA) {
		partsB = append(partsB, "")
	}
	for i := range partsA {
		if partsA[i] == partsB[i] {
			continue
		}
		if partsA[i] == "" {
			// incomparable part, no conflict
			continue
		}
		if partsB[i] == "" {
			// incomparable part, no conflict
			continue
		}
		// Parts are not equal and neither of them is "" -> conflict
		return true
	}
	return false
}
