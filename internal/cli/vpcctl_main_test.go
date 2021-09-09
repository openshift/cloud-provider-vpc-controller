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

package cli

import (
	"testing"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpclb"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	mockSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "storage-secret-store", Namespace: "kube-system"},
		Data: map[string][]byte{"slclient.toml": []byte(`[VPC]
	provider_type = "fake"`)}}
	mockService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "1234"}}
	mockSubnetMap = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: vpclb.VpcCloudProviderConfigMap, Namespace: vpclb.VpcCloudProviderNamespace},
		Data:       map[string]string{vpclb.VpcCloudProviderSubnetsKey: "subnetID"}}
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

	// No secret
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap)
	err = CreateLoadBalancer("echo-server", "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get secret")

	// Valid config map / secret - service not found
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService)
	err = CreateLoadBalancer("lbName", "invalid-service")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get service")

	// Valid config map / secret - create failed
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService, mockSubnetMap)
	err = CreateLoadBalancer("echo-server", "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no available nodes for this service")

	// Valid config map / secret - create worked
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService, mockNode, mockSubnetMap)
	err = CreateLoadBalancer("echo-server", "")
	assert.Nil(t, err)

	// Valid config map / secret - LB already exists, but it is not ready
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService, mockNode, mockSubnetMap)
	err = CreateLoadBalancer("NotReady", "echo-server")
	assert.Nil(t, err)

	// Valid config map / secret - LB exists, update LB failed. No nodes
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService, mockSubnetMap)
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

	// No secret
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap)
	err = DeleteLoadBalancer("load balancer")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get secret")

	// Valid config map / secret - LB not found
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService)
	err = DeleteLoadBalancer("LB not found")
	assert.Nil(t, err)

	// Valid config map / secret - LB not found, Kube service passed in
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService)
	err = DeleteLoadBalancer("echo-server")
	assert.Nil(t, err)

	// Valid config map / secret - LB found, not ready
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService)
	err = DeleteLoadBalancer("NotReady")
	assert.Nil(t, err)

	// Valid config map / secret - LB found, Ready, Deleted
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, mockService)
	err = DeleteLoadBalancer("Ready")
	assert.Nil(t, err)
}

func TestGetCloudProviderVpc(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// No config map, no secret
	mockKubeCtl = fake.NewSimpleClientset()
	c, err := GetCloudProviderVpc()
	assert.Nil(t, c)
	assert.NotNil(t, err)
}

func TestMonitorLoadBalancers(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// No secret
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap)
	err := MonitorLoadBalancers()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get secret")

	// Success
	serviceNodePort := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nodePort", Namespace: "default", UID: "NodePort"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeNodePort}}
	serviceNotFound := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notFound", Namespace: "default", UID: "NotFound"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	serviceNotReady := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notReady", Namespace: "default", UID: "NotReady"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret, serviceNodePort, serviceNotFound, serviceNotReady)
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

	// No secret
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap)
	err = StatusLoadBalancer("LB")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get secret")

	// Valid config map / secret
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap, mockSecret)
	err = StatusLoadBalancer("LB not found")
	assert.Nil(t, err)

	// Retrieve status of LB named: "Ready"
	err = StatusLoadBalancer("Ready")
	assert.Nil(t, err)

	// Retrieve status of LB named: "NotReady"
	err = StatusLoadBalancer("NotReady")
	assert.Nil(t, err)
}
