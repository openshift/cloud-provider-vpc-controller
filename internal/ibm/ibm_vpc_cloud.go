/*******************************************************************************
* IBM Cloud Kubernetes Service, 5737-D43
* (C) Copyright IBM Corp. 2021 All Rights Reserved.
*
* SPDX-License-Identifier: Apache2.0
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*******************************************************************************/

package ibm

import (
	"fmt"
	"os"
	"strings"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpcctl"
	"gopkg.in/gcfg.v1"
)

// InitCloudVpc - Initialize the VPC cloud logic
func (c *Cloud) InitCloudVpc(enablePrivateEndpoint bool) (*vpcctl.CloudVpc, error) {
	// Extract the VPC cloud object. If set, return it
	cloudVpc := c.Vpc
	if cloudVpc != nil {
		return cloudVpc, nil
	}
	// Initialize options based on values in the cloud provider
	options, err := c.NewCloudVpcOptions(enablePrivateEndpoint)
	if err != nil {
		return nil, err
	}
	// Allocate a new VPC Cloud object and save it if successful
	cloudVpc, err = vpcctl.NewCloudVpc(c.KubeClient, options)
	if cloudVpc != nil {
		c.Vpc = cloudVpc
	}
	return cloudVpc, err
}

// NewCloudVpcOptions - Create the CloudVpcOptions from the current Cloud object
func (c *Cloud) NewCloudVpcOptions(enablePrivateEndpoint bool) (*vpcctl.CloudVpcOptions, error) {
	// Make sure Cloud config has been initialized
	if c.Config == nil {
		return nil, fmt.Errorf("Cloud config not initialized")
	}
	// Initialize options based on values in the cloud provider
	options := &vpcctl.CloudVpcOptions{
		ClusterID:         c.Config.Prov.ClusterID,
		EnablePrivate:     enablePrivateEndpoint,
		ProviderType:      c.Config.Prov.ProviderType,
		Region:            c.Config.Prov.Region,
		ResourceGroupName: c.Config.Prov.G2ResourceGroupName,
		SubnetNames:       c.Config.Prov.G2VpcSubnetNames,
		WorkerAccountID:   c.Config.Prov.G2WorkerServiceAccountID,
		VpcName:           c.Config.Prov.G2VpcName,
	}
	// If the G2Credentials is set, then look up the API key
	if c.Config.Prov.G2Credentials != "" {
		apiKey, err := os.ReadFile(c.Config.Prov.G2Credentials)
		if err != nil {
			return nil, fmt.Errorf("Failed to read credentials from %s: %v", c.Config.Prov.G2Credentials, err)
		}
		options.APIKey = strings.TrimSpace(string(apiKey))
	}
	return options, nil
}

// ReadCloudConfig - Read in the cloud configuration
func (c *Cloud) ReadCloudConfig(configFile string) (*CloudConfig, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("missing file: %s", configFile)
	}
	var config CloudConfig
	err := gcfg.FatalOnly(gcfg.ReadFileInto(&config, configFile))
	if err != nil {
		return nil, fmt.Errorf("fatal error occurred processing file: %s", configFile)
	}
	return &config, nil
}
