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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCloud_InitCloudVpc(t *testing.T) {
	c := Cloud{Config: &CloudConfig{Prov: Provider{ClusterID: "clusterID"}}, KubeClient: fake.NewSimpleClientset()}
	v, err := c.InitCloudVpc(true)
	assert.Nil(t, v)
	assert.NotNil(t, err)
}

func TestCloud_NewConfigVpc(t *testing.T) {
	// Test for the case of cloud config not initialized
	c := Cloud{}
	config, err := c.NewConfigVpc(true)
	assert.Nil(t, config)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Cloud config not initialized")

	// Test failure to read credentials from file
	c.Config = &CloudConfig{Prov: Provider{
		Region:                   "us-south",
		ClusterID:                "clusterID",
		ProviderType:             "g2",
		G2Credentials:            "../../test-fixtures/missing-file.txt",
		G2ResourceGroupName:      "Default",
		G2VpcSubnetNames:         "subnet1,subnet2,subnet3",
		G2WorkerServiceAccountID: "accountID",
		G2VpcName:                "vpc",
	}}
	config, err = c.NewConfigVpc(true)
	assert.Nil(t, config)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to read credentials")

	// Successfully return CloudVpcOptions
	c.Config.Prov.G2Credentials = "../../test-fixtures/creds.txt"
	config, err = c.NewConfigVpc(true)
	assert.NotNil(t, config)
	assert.Nil(t, err)
	assert.Equal(t, config.APIKeySecret, "apiKey-1234")
	assert.Equal(t, config.ClusterID, "clusterID")
	assert.Equal(t, config.EnablePrivate, true)
	assert.Equal(t, config.ProviderType, "g2")
	assert.Equal(t, config.Region, "us-south")
	assert.Equal(t, config.ResourceGroupName, "Default")
	assert.Equal(t, config.SubnetNames, "subnet1,subnet2,subnet3")
	assert.Equal(t, config.WorkerAccountID, "accountID")
	assert.Equal(t, config.VpcName, "vpc")
}

func TestCloud_ReadCloudConfig(t *testing.T) {
	c := Cloud{}
	// Test for the case of missing config file
	config, err := c.ReadCloudConfig("../../test-fixtures/cloud-conf-missing.ini")
	assert.Nil(t, config)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "missing file")

	// Test for the case of missing config file
	config, err = c.ReadCloudConfig("../../test-fixtures/cloud-conf-error.ini")
	assert.Nil(t, config)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Test for the reading in the g2 config file
	config, err = c.ReadCloudConfig("../../test-fixtures/cloud-conf-g2.ini")
	assert.NotNil(t, config)
	assert.Nil(t, err)
	assert.Equal(t, config.Prov.ClusterID, "clusterID")
	assert.Equal(t, config.Prov.ProviderType, "g2")
	assert.Equal(t, config.Prov.Region, "us-south")
	assert.Equal(t, config.Prov.G2Credentials, "../../test-fixtures/creds.txt")
	assert.Equal(t, config.Prov.G2ResourceGroupName, "Default")
	assert.Equal(t, config.Prov.G2VpcSubnetNames, "subnet1,subnet2,subnet3")
	assert.Equal(t, config.Prov.G2VpcName, "vpc")
}
