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
)

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
