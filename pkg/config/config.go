/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package config

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	VirtioFSImage    string
	ResourceRequests struct {
		CPU    resource.Quantity
		Memory resource.Quantity
	}
	ResourceLimits struct {
		CPU    resource.Quantity
		Memory resource.Quantity
	}
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.VirtioFSImage = getEnvOrDefault("VIRTIOFS_IMAGE", "quay.io/kubevirt/virt-launcher:v1.5.1")

	cfg.ResourceRequests.CPU = resource.MustParse(getEnvOrDefault("RESOURCE_REQUESTS_CPU", "100m"))
	cfg.ResourceRequests.Memory = resource.MustParse(getEnvOrDefault("RESOURCE_REQUESTS_MEMORY", "128Mi"))

	cfg.ResourceLimits.CPU = resource.MustParse(getEnvOrDefault("RESOURCE_LIMITS_CPU", "200m"))
	cfg.ResourceLimits.Memory = resource.MustParse(getEnvOrDefault("RESOURCE_LIMITS_MEMORY", "256Mi"))

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
