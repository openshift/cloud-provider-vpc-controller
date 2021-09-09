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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	mockClusterID = "clusterID"
	mockConfigMap = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigMap, Namespace: clusterNamespace},
		Data:       map[string]string{clusterConfigKey: `{ "cluster_id": "clusterID" }`}}
	mockKubeCtl    = fake.NewSimpleClientset()
	mockKubeCtlErr error
)

func mockGetKubectl() (kubernetes.Interface, error) {
	return mockKubeCtl, mockKubeCtlErr
}

func TestCloudInit(t *testing.T) {
	// Return mock kubectl
	getKubernetesClient = mockGetKubectl

	// Fail to get kubernetes client
	mockKubeCtlErr = fmt.Errorf("mock failure")
	client, clusterID, err := cloudInit()
	mockKubeCtlErr = nil
	assert.Nil(t, client)
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "mock failure")

	// Config map for the ClusterID does not exist
	mockKubeCtl = fake.NewSimpleClientset()
	client, clusterID, err = cloudInit()
	assert.NotNil(t, client)
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to get kube-system/cluster-info config map")

	// Config map for the ClusterID does not exist
	mockKubeCtl = fake.NewSimpleClientset(mockConfigMap)
	client, clusterID, err = cloudInit()
	assert.NotNil(t, client)
	assert.Equal(t, clusterID, mockClusterID)
	assert.Nil(t, err)
}
func TestGetClusterIDFromINIFile(t *testing.T) {
	// The INI file does not exist
	clusterID, err := getClusterIDFromINIFile("../../test-fixtures/ibm-cloud-config-nosuchfile.ini")
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "missing INI file")

	// The INI format is invalid, so it causes fatal error
	clusterID, err = getClusterIDFromINIFile("../../test-fixtures/ibm-cloud-config-error.ini")
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fatal error occurs during the INI file processing")

	// The clusterID key is missing from INI file
	clusterID, err = getClusterIDFromINIFile("../../test-fixtures/ibm-cloud-config-missing.ini")
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "missing or empty ClusterID in INI file")

	// The clusterID value is empty in INI file
	clusterID, err = getClusterIDFromINIFile("../../test-fixtures/ibm-cloud-config-empty.ini")
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "missing or empty ClusterID in INI file")

	// Success
	clusterID, err = getClusterIDFromINIFile("../../test-fixtures/ibm-cloud-config-good.ini")
	assert.Equal(t, clusterID, "testclusterID")
	assert.Nil(t, err)
}
func TestGetClusterID(t *testing.T) {
	// Config map does not exist
	client := fake.NewSimpleClientset()
	clusterID, err := getClusterID(client)
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to get kube-system/cluster-info config map")

	// Config map is empty
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigMap, Namespace: clusterNamespace},
	}
	client = fake.NewSimpleClientset(cm)
	clusterID, err = getClusterID(client)
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "kube-system/cluster-info config map does not contain key")

	// Config map contains bad json data
	cm = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigMap, Namespace: clusterNamespace},
		Data:       map[string]string{clusterConfigKey: "bad json data"},
	}
	client = fake.NewSimpleClientset(cm)
	clusterID, err = getClusterID(client)
	assert.Equal(t, clusterID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to un-marshall config data")

	// Success
	cm = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigMap, Namespace: clusterNamespace},
		Data:       map[string]string{clusterConfigKey: `{ "cluster_id": "bk97be720oupmg8m9m30" }`},
	}
	client = fake.NewSimpleClientset(cm)
	clusterID, err = getClusterID(client)
	assert.Equal(t, clusterID, "bk97be720oupmg8m9m30")
	assert.Nil(t, err)
}

func TestGetKubectl(t *testing.T) {
	// KUBECONFIG set to non-existent file
	os.Setenv("KUBECONFIG", "/tmp/invalid/subdir/non-existant-kubeconfig.cfg")
	kubectl, err := getKubectl()
	assert.Nil(t, kubectl)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
	os.Unsetenv("KUBECONFIG")

	// KUBECONFIG set to an invalid file
	fileName := "/tmp/tmp-kubeconfig.cfg"
	_, err = os.Create(fileName)
	assert.Nil(t, err)
	os.Setenv("KUBECONFIG", fileName)
	kubectl, err = getKubectl()
	assert.Nil(t, kubectl)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid configuration: no configuration has been provided")
	os.Unsetenv("KUBECONFIG")
	os.Remove(fileName)

	// KUBECONFIG not set, default KUBECONFIG is searched for, but not found
	os.Setenv("HOME", "/tmp/invalid")
	kubectl, err = getKubectl()
	assert.Nil(t, kubectl)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	os.Unsetenv("HOME")
}

func TestGetKubeNodes(t *testing.T) {
	// No nodes in the list
	client := fake.NewSimpleClientset()
	nodes, err := getKubeNodes(client)
	assert.Nil(t, err)
	assert.Equal(t, len(nodes), 0)

	// Both of the nodes "Ready"
	mockNode1 := v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "192.168.1.1"},
		Status:     v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}},
	}
	mockNode2 := v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "192.168.2.2"},
		Status:     v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}},
	}
	client = fake.NewSimpleClientset(&mockNode1, &mockNode2)
	nodes, err = getKubeNodes(client)
	assert.Nil(t, err)
	assert.Equal(t, len(nodes), 2)

	// Cordon the 2nd node
	mockNode2.Spec.Unschedulable = true
	client = fake.NewSimpleClientset(&mockNode1, &mockNode2)
	nodes, err = getKubeNodes(client)
	assert.Nil(t, err)
	assert.Equal(t, len(nodes), 1)

	// Remove the 2nd node, add a node that is not "Ready"
	mockNode3 := v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.2.2"}}
	client = fake.NewSimpleClientset(&mockNode1, &mockNode3)
	nodes, err = getKubeNodes(client)
	assert.Nil(t, err)
	assert.Equal(t, len(nodes), 1)
}

func TestGetKubeService(t *testing.T) {
	// Service does not exist
	client := fake.NewSimpleClientset()
	service, err := getKubeService(client, "echo-server")
	assert.Nil(t, service)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get service")

	// Success
	kubeService := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	client = fake.NewSimpleClientset(kubeService)
	service, err = getKubeService(client, "echo-server")
	assert.NotNil(t, service)
	assert.Nil(t, err)
}

func TestGetKubeServices(t *testing.T) {
	// No services exist
	client := fake.NewSimpleClientset()
	services, err := getKubeServices(client)
	assert.Nil(t, err)
	assert.NotNil(t, services)
	assert.Equal(t, len(services.Items), 0)

	// Success - 1 service retrieved
	kubeService := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	client = fake.NewSimpleClientset(kubeService)
	services, err = getKubeServices(client)
	assert.Nil(t, err)
	assert.NotNil(t, services)
	assert.Equal(t, len(services.Items), 1)
}

func TestShouldPrivateEndpointBeEnabled(t *testing.T) {
	// Running on MacOS
	runtimeOS = "darwin"
	result := shouldPrivateEndpointBeEnabled()
	assert.False(t, result)

	// Running on Linux
	runtimeOS = "linux"
	result = shouldPrivateEndpointBeEnabled()
	assert.True(t, result)

	// Set Linux override env variable
	os.Setenv(envVarPublicEndPoint, "true")
	defer os.Unsetenv(envVarPublicEndPoint)
	result = shouldPrivateEndpointBeEnabled()
	assert.False(t, result)
}
