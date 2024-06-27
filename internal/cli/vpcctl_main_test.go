/*******************************************************************************
* IBM Cloud Kubernetes Service, 5737-D43
* (C) Copyright IBM Corp. 2021, 2022 All Rights Reserved.
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

package cli

import (
	"os"
	"testing"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpcctl"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	mockService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "1234"}}
)

const (
	cloudConfPathError = "../../test-fixtures/cloud-conf-error.ini"
	cloudConfPathFake  = "../../test-fixtures/cloud-conf-fake.ini"
)

func TestCreateLoadBalancer(t *testing.T) {
	mockNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "192.168.1.1"},
		Status:     v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}},
	}
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// Argument not specified
	err := CreateLoadBalancer("", "")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Required argument is missing")

	// Invalid configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathError)
	defer os.Unsetenv(envVarCloudConfigPath)
	mockKubeCtl = fake.NewSimpleClientset()
	err = CreateLoadBalancer("echo-server", "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Valid configuration - service not found
	os.Setenv(envVarCloudConfigPath, cloudConfPathFake)
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = CreateLoadBalancer("lbName", "invalid-service")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get service")

	// Valid configuration - create failed
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = CreateLoadBalancer("echo-server", "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no available nodes for this service")

	// Valid configuration - create worked
	vpcctl.ResetCloudVpc()
	mockKubeCtl = fake.NewSimpleClientset(mockService, mockNode)
	err = CreateLoadBalancer("echo-server", "")
	assert.Nil(t, err)

	// Valid configuration - LB already exists, but it is not ready
	mockKubeCtl = fake.NewSimpleClientset(mockService, mockNode)
	err = CreateLoadBalancer("NotReady", "echo-server")
	assert.Nil(t, err)

	// Valid configuration - LB exists, update LB failed. No nodes
	vpcctl.ResetCloudVpc()
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = CreateLoadBalancer("Ready", "echo-server")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no available nodes")
}

func TestDeleteLoadBalancer(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// Argument not specified
	err := DeleteLoadBalancer("")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Required argument is missing")

	// No configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathError)
	defer os.Unsetenv(envVarCloudConfigPath)
	mockKubeCtl = fake.NewSimpleClientset()
	err = DeleteLoadBalancer("load balancer")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Valid configuration - LB not found
	os.Setenv(envVarCloudConfigPath, cloudConfPathFake)
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = DeleteLoadBalancer("LB not found")
	assert.Nil(t, err)

	// Valid configuration - LB not found, Kube service passed in
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = DeleteLoadBalancer("echo-server")
	assert.Nil(t, err)

	// Valid configuration - LB found, Ready, Deleted
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = DeleteLoadBalancer("Ready")
	assert.Nil(t, err)
}

func TestGetCloudProviderVpc(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// Environment variable not set
	mockKubeCtl = fake.NewSimpleClientset()
	c, err := GetCloudProviderVpc()
	assert.Nil(t, c)
	assert.NotNil(t, err)
}

func TestMonitorLoadBalancers(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// No configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathError)
	defer os.Unsetenv(envVarCloudConfigPath)
	mockKubeCtl = fake.NewSimpleClientset()
	err := MonitorLoadBalancers()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Success
	os.Setenv(envVarCloudConfigPath, cloudConfPathFake)
	serviceNodePort := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nodePort", Namespace: "default", UID: "NodePort"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeNodePort}}
	serviceNotFound := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notFound", Namespace: "default", UID: "NotFound"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	serviceNotReady := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notReady", Namespace: "default", UID: "NotReady"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	mockKubeCtl = fake.NewSimpleClientset(serviceNodePort, serviceNotFound, serviceNotReady)
	err = MonitorLoadBalancers()
	assert.Nil(t, err)
}

func TestStatusLoadBalancer(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// Argument not specified
	err := StatusLoadBalancer("")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Required argument is missing")

	// No configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathError)
	defer os.Unsetenv(envVarCloudConfigPath)
	mockKubeCtl = fake.NewSimpleClientset()
	err = StatusLoadBalancer("LB")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Valid configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathFake)
	err = StatusLoadBalancer("LB not found")
	assert.Nil(t, err)

	// Retrieve status of LB named: "Ready"
	err = StatusLoadBalancer("Ready")
	assert.Nil(t, err)

	// Retrieve status of LB named: "NotReady"
	err = StatusLoadBalancer("NotReady")
	assert.Nil(t, err)
}

func TestUpdateLoadBalancer(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// // Argument not specified
	err := UpdateLoadBalancer("", "")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Required argument is missing")

	// No configuration
	os.Setenv(envVarCloudConfigPath, cloudConfPathError)
	defer os.Unsetenv(envVarCloudConfigPath)
	mockKubeCtl = fake.NewSimpleClientset()
	err = UpdateLoadBalancer("lbName", "echo-server")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurred processing file")

	// Valid configuration - service not found
	os.Setenv(envVarCloudConfigPath, cloudConfPathFake)
	mockKubeCtl = fake.NewSimpleClientset(mockService)
	err = UpdateLoadBalancer("lbName", "invalid-service")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get service")

	// Valid configuration/service - failed to find load balancer
	err = UpdateLoadBalancer("lbName", "echo-server")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Valid configuration/service - LB exists, but not ready
	err = UpdateLoadBalancer("NotReady", "echo-server")
	assert.Nil(t, err)

	// Valid configuration/service - LB exists, update failed.  No nodes
	err = UpdateLoadBalancer("Ready", "echo-server")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no available nodes")
}
