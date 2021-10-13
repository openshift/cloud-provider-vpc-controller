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

package vpcctl

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	cluster = "bqcssbbd0bsui62odcdg"
)

var gen2Data = `[VPC]
g2_riaas_endpoint_url = "https://us-south.iaas.cloud.ibm.com:443"
g2_riaas_endpoint_private_url = "https://private-us-south.iaas.cloud.ibm.com:443"
g2_resource_group_id = "resourceGroup"
g2_api_key = "foobar"
provider_type = "g2"
iks_token_exchange_endpoint_private_url = "https://private.us-south.containers.cloud.ibm.com"`
var gen2CloudVpc = &CloudVpc{
	Config: ConfigVpc{
		APIKeySecret:     "foobar",
		ClusterID:        cluster,
		EnablePrivate:    true,
		EndpointURL:      "https://private-us-south.iaas.cloud.ibm.com:443/v1",
		ProviderType:     "g2",
		ResourceGroupID:  "resourceGroup",
		TokenExchangeURL: "https://private.iam.cloud.ibm.com/identity/token",
	}}

var mockCloud = CloudVpc{KubeClient: fake.NewSimpleClientset()}

// Node without InternalIP label but with status
var mockNode1 = &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.1.1",
	Labels: map[string]string{nodeLabelZone: "zoneA", nodeLabelDedicated: nodeLabelValueEdge}}, Status: v1.NodeStatus{Addresses: []v1.NodeAddress{{Address: "192.168.1.1", Type: v1.NodeInternalIP}}}}

// Node with InteralIP label but without status
var mockNode2 = &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.2.2",
	Labels: map[string]string{nodeLabelZone: "zoneB", nodeLabelInternalIP: "192.168.2.2"}}}

// Node without InternalIP label and status
var mockNode3 = &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.3.3",
	Labels: map[string]string{nodeLabelZone: "zoneB"}}}

// Node without InternalIP label with nil Addresses status
var mockNode4 = &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.1.1",
	Labels: map[string]string{nodeLabelZone: "zoneA", nodeLabelDedicated: nodeLabelValueEdge}}, Status: v1.NodeStatus{Addresses: nil}}

func getSecretNotFound() kubernetes.Interface {
	return fake.NewSimpleClientset()
}
func getSecretData(secretData string) kubernetes.Interface {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: VpcSecretFileName, Namespace: VpcSecretNamespace},
		Data:       map[string][]byte{VpcClientDataKey: []byte(secretData)},
	}
	return fake.NewSimpleClientset(secret)
}

func TestNewCloudVpc(t *testing.T) {
	type args struct {
		kubeClient            kubernetes.Interface
		clusterID             string
		enablePrivateEndpoint bool
	}
	tests := []struct {
		name    string
		args    args
		want    *CloudVpc
		wantErr bool
	}{
		{
			name: "No secret",
			args: args{kubeClient: getSecretNotFound(), clusterID: cluster, enablePrivateEndpoint: false},
			want: nil, wantErr: true,
		},
		{
			name: "No [VPC] data in the secret",
			args: args{kubeClient: getSecretData("Secret Data"), clusterID: cluster, enablePrivateEndpoint: false},
			want: nil, wantErr: true,
		},
		{
			name: "No API Key in the secret",
			args: args{kubeClient: getSecretData("[VPC]"), clusterID: cluster, enablePrivateEndpoint: false},
			want: nil, wantErr: true,
		},
		{
			name: "Valid Gen2 secret - encrypted / private service endpoint",
			args: args{kubeClient: getSecretData(gen2Data), clusterID: cluster, enablePrivateEndpoint: true},
			want: gen2CloudVpc, wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &CloudVpcOptions{
				ClusterID:       tt.args.clusterID,
				EnablePrivate:   tt.args.enablePrivateEndpoint,
				WorkerAccountID: "workerAccountID",
			}
			got, err := NewCloudVpc(tt.args.kubeClient, options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCloudVpc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// if got != nil && tt.want != nil && !equalCloudVpc(got, tt.want) {
			if got != nil && tt.want != nil && !reflect.DeepEqual(got.Config, tt.want.Config) {
				t.Errorf("NewCloudVpc()\ngot = %+v\nwant = %+v", got.Config, tt.want.Config)
			}
		})
	}
}

func TestConfigVpc_GetSummary(t *testing.T) {
	config := ConfigVpc{
		ClusterID:        "clusterID",
		EndpointURL:      "https://us-south.iaas.cloud.ibm.com:443/v1",
		ProviderType:     "g2",
		ResourceGroupID:  "resourceGroupID",
		TokenExchangeURL: "https://iam.bluemix.net",
	}
	result := config.GetSummary()
	assert.Equal(t, result, "ClusterID:clusterID Endpoint:https://us-south.iaas.cloud.ibm.com:443/v1 Provider:g2 ResourceGroup:resourceGroupID TokenExchangeURL:https://iam.bluemix.net")
}

func TestCloudVpc_filterLoadBalancersOnlyNLB(t *testing.T) {
	lb1 := &VpcLoadBalancer{ID: "lb1", Name: "kube-clusterID-1234", ProfileFamily: "network"}
	lb2 := &VpcLoadBalancer{ID: "lb2", Name: "kube-clusterID-1234"}

	// Find the one NLB node in the list
	inLBs := []*VpcLoadBalancer{lb1, lb2}
	outLBs := mockCloud.filterLoadBalancersOnlyNLB(inLBs)
	assert.Equal(t, len(outLBs), 1)
	assert.Equal(t, outLBs[0].ID, "lb1")
}

func TestCloudVpc_FilterNodesByEdgeLabel(t *testing.T) {
	// Pull out the 1 edge node from the list of 2 nodes
	inNodes := []*v1.Node{mockNode1, mockNode2}
	outNodes := mockCloud.filterNodesByEdgeLabel(inNodes)
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode1.Name)

	// No edge nodes in the list
	inNodes = []*v1.Node{mockNode2}
	outNodes = mockCloud.filterNodesByEdgeLabel(inNodes)
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode2.Name)
}

func TestCloudVpc_filterNodesByNodeNames(t *testing.T) {
	// Pull out the 1 node that is in the map
	nodeNames := map[string]int{"192.168.1.1": 1, "192.168.2.2": 0}
	inNodes := []*v1.Node{mockNode1, mockNode2}
	outNodes := mockCloud.filterNodesByNodeNames(inNodes, nodeNames)
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode1.Name)

	// No nodes are listed in the map
	nodeNames = map[string]int{"192.168.1.1": 0, "192.168.2.2": 0, "192.168.3.3": 0}
	inNodes = []*v1.Node{mockNode1, mockNode2}
	outNodes = mockCloud.filterNodesByNodeNames(inNodes, nodeNames)
	assert.Equal(t, len(outNodes), 0)
}

func TestCloudVpc_FilterNodesByServiceMemberQuota(t *testing.T) {
	mockService := &v1.Service{}
	desiredNodes := []string{"192.168.1.1", "192.168.2.2", "192.168.3.3", "192.168.4.4"}
	existingNodes := []string{"192.168.2.2", "192.168.5.5", "192.168.6.6"}
	// Invalid annotation on the service
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "invalid"}
	nodes, err := mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Nil(t, nodes)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "is not set to a valid value")

	// Disable quota checking annotation on the service
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "disable"}
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), len(desiredNodes))
	assert.Nil(t, err)

	// Number of nodes is less than the service quota
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "10"}
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), len(desiredNodes))
	assert.Nil(t, err)

	// ExternalTrafficPolicy: Local and we are over the quota. All desired nodes are returned
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "2"}
	mockService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), len(desiredNodes))
	assert.Nil(t, err)
	mockService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster

	// ExternalTrafficPolicy: Cluster and we are over the quota
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "1"}
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), 1)
	assert.Nil(t, err)
	assert.Equal(t, nodes[0], "192.168.2.2")

	// ExternalTrafficPolicy: Cluster and we are over the quota
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "2"}
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), 2)
	assert.Nil(t, err)
	assert.Equal(t, nodes[0], "192.168.2.2")
	assert.Equal(t, nodes[1], "192.168.1.1")

	// ExternalTrafficPolicy: Cluster and we are over the quota
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "3"}
	nodes, err = mockCloud.filterNodesByServiceMemberQuota(desiredNodes, existingNodes, mockService)
	assert.Equal(t, len(nodes), 3)
	assert.Nil(t, err)
	assert.Equal(t, nodes[0], "192.168.2.2")
	assert.Equal(t, nodes[1], "192.168.1.1")
	assert.Equal(t, nodes[2], "192.168.3.3")
}

func TestCloudVpc_FilterNodesByServiceZone(t *testing.T) {
	// No annotation on the service, match both of the nodes
	mockService := &v1.Service{}
	inNodes := []*v1.Node{mockNode1, mockNode2}
	outNodes := mockCloud.filterNodesByServiceZone(inNodes, mockService)
	assert.Equal(t, len(outNodes), 2)

	// Add the zone annotation to the service, re-calc matching nodes
	mockService.Annotations = map[string]string{serviceAnnotationZone: "zoneA"}
	outNodes = mockCloud.filterNodesByServiceZone(inNodes, mockService)
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode1.Name)
}

func TestCloudVpc_filterNodesByZone(t *testing.T) {
	// No nodes matching the request zone
	inNodes := []*v1.Node{mockNode1, mockNode2}
	outNodes := mockCloud.filterNodesByZone(inNodes, "zoneX")
	assert.Equal(t, len(outNodes), 0)

	// Find nodes matching one of the zones
	outNodes = mockCloud.filterNodesByZone(inNodes, "zoneA")
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode1.Name)
}

func TestCloudVpc_filterSubnetsBySubnetIDs(t *testing.T) {
	// No subnets matching the requested subnet ID
	inSubnets := []*VpcSubnet{{ID: "subnet1"}, {ID: "subnet2"}, {ID: "subnet3"}}
	outSubnets := mockCloud.filterSubnetsBySubnetIDs(inSubnets, []string{"subnet"})
	assert.Equal(t, len(outSubnets), 0)

	// Find the two subnets that match
	outSubnets = mockCloud.filterSubnetsBySubnetIDs(inSubnets, []string{"subnet1", "subnet3"})
	assert.Equal(t, len(outSubnets), 2)
	assert.Equal(t, outSubnets[0].ID, "subnet1")
	assert.Equal(t, outSubnets[1].ID, "subnet3")
}

func TestCloudVpc_filterSubnetsByZone(t *testing.T) {
	// No subnets matching the requested zone
	inSubnets := []*VpcSubnet{{ID: "subnet1", Zone: "zoneA"}, {ID: "subnet2", Zone: "zoneB"}, {ID: "subnet3", Zone: "zoneA"}}
	outSubnets := mockCloud.filterSubnetsByZone(inSubnets, "zone")
	assert.Equal(t, len(outSubnets), 0)

	// Find the two subnets that match
	outSubnets = mockCloud.filterSubnetsByZone(inSubnets, "zoneA")
	assert.Equal(t, len(outSubnets), 2)
	assert.Equal(t, outSubnets[0].ID, "subnet1")
	assert.Equal(t, outSubnets[1].ID, "subnet3")
}

func TestCloudVpc_filterZonesByNodeCountsInEachZone(t *testing.T) {
	// None of the input subnets have nodes
	nodeCounts := map[string]int{"zoneA": 1, "zoneB": 3, "zoneC": 2}
	inZones := []string{"zoneX", "zoneY", "zoneZ"}
	outZones := mockCloud.filterZonesByNodeCountsInEachZone(inZones, nodeCounts)
	assert.Equal(t, len(outZones), 0)

	// Two of subnets have worker nodes
	inZones = []string{"zoneA", "zoneB", "zoneX", "zoneY", "zoneZ"}
	outZones = mockCloud.filterZonesByNodeCountsInEachZone(inZones, nodeCounts)
	assert.Equal(t, len(outZones), 2)
	assert.Equal(t, outZones[0], "zoneA")
	assert.Equal(t, outZones[1], "zoneB")
}

func TestCloudVpc_FindNodesMatchingLabelValue(t *testing.T) {
	// Pull out the 1 edge node from the list of 2 nodes
	inNodes := []*v1.Node{mockNode1, mockNode2}
	outNodes := mockCloud.findNodesMatchingLabelValue(inNodes, nodeLabelDedicated, nodeLabelValueEdge)
	assert.Equal(t, len(outNodes), 1)
	assert.Equal(t, outNodes[0].Name, mockNode1.Name)

	// No edge nodes in the list, return matches = 0
	inNodes = []*v1.Node{mockNode2}
	outNodes = mockCloud.findNodesMatchingLabelValue(inNodes, nodeLabelDedicated, nodeLabelValueEdge)
	assert.Equal(t, len(outNodes), 0)
}

func TestCloudVpc_findServiceByLbName(t *testing.T) {
	// Find service, service not found
	c := CloudVpc{KubeClient: fake.NewSimpleClientset(), Config: ConfigVpc{ClusterID: "clusterID"}}
	service, err := c.findServiceByLbName("service not found")
	assert.Nil(t, service)
	assert.Nil(t, err)

	// Find service, service found
	mockService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "1234", Annotations: map[string]string{}},
		Spec:       v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer},
	}
	c.KubeClient = fake.NewSimpleClientset(mockService)
	service, err = c.findServiceByLbName("kube-clusterID-1234")
	assert.NotNil(t, service)
	assert.Nil(t, err)
}

func TestCloudVpc_GenerateLoadBalancerName(t *testing.T) {
	clusterID := "12345678901234567890"
	c := &CloudVpc{
		Config: ConfigVpc{ClusterID: clusterID},
	}
	kubeService := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "echo-server", Namespace: "default", UID: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"}}
	lbName := VpcLbNamePrefix + "-" + clusterID + "-" + string(kubeService.UID)
	lbName = lbName[:63]
	result := c.GenerateLoadBalancerName(kubeService)
	assert.Equal(t, result, lbName)
}

func TestCloudVpc_GetClusterSubnets(t *testing.T) {
	c := CloudVpc{KubeClient: fake.NewSimpleClientset()}
	vpcID, subnets, err := c.GetClusterVpcSubnetIDs()
	assert.Equal(t, vpcID, "")
	assert.Equal(t, len(subnets), 0)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("Failed to get %s/%s config map", VpcCloudProviderNamespace, VpcCloudProviderConfigMap))

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: VpcCloudProviderConfigMap, Namespace: VpcCloudProviderNamespace}}
	c.KubeClient = fake.NewSimpleClientset(configMap)
	vpcID, subnets, err = c.GetClusterVpcSubnetIDs()
	assert.Equal(t, vpcID, "")
	assert.Equal(t, len(subnets), 0)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "config map does not contain key")

	configMap.Data = map[string]string{VpcCloudProviderSubnetsKey: "f16dd75c-dce9-4724-bab4-59db6aa2300a", VpcCloudProviderVpcIDKey: "1234-5678"}
	c.KubeClient = fake.NewSimpleClientset(configMap)
	vpcID, subnets, err = c.GetClusterVpcSubnetIDs()
	assert.Equal(t, vpcID, "1234-5678")
	assert.Equal(t, len(subnets), 1)
	assert.Equal(t, subnets[0], "f16dd75c-dce9-4724-bab4-59db6aa2300a")
	assert.Nil(t, err)
}

func TestCloudVpc_GetClusterVpcID(t *testing.T) {
	// Failed to find VPC that matches the VPC ID passed in
	sdk, _ := NewVpcSdkFake()
	c := CloudVpc{
		Sdk:        sdk,
		KubeClient: fake.NewSimpleClientset(),
	}
	vpcID, err := c.GetClusterVpcID()
	assert.Equal(t, vpcID, "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("Failed to get %s/%s config map", VpcCloudProviderNamespace, VpcCloudProviderConfigMap))
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: VpcCloudProviderConfigMap, Namespace: VpcCloudProviderNamespace},
		Data:       map[string]string{VpcCloudProviderSubnetsKey: "subnetID"},
	}
	c.KubeClient = fake.NewSimpleClientset(configMap)

	// Failed to get list of VPCs
	c.SetFakeSdkError("GetSubnet")
	vpcID, err = c.GetClusterVpcID()
	assert.Equal(t, vpcID, "")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "GetSubnet failed")
	c.ClearFakeSdkError("GetSubnet")

	// Success - found the VPC ID by comparing cluster subnets and actual subnets
	vpcID, err = c.GetClusterVpcID()
	assert.Equal(t, vpcID, "vpcID")
	assert.Nil(t, err)

	// Success - VPC ID is stored in the config map
	configMap = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: VpcCloudProviderConfigMap, Namespace: VpcCloudProviderNamespace},
		Data:       map[string]string{VpcCloudProviderSubnetsKey: "subnetID", VpcCloudProviderVpcIDKey: "vpcFromConfigMap"},
	}
	c.KubeClient = fake.NewSimpleClientset(configMap)
	vpcID, err = c.GetClusterVpcID()
	assert.Equal(t, vpcID, "vpcFromConfigMap")
	assert.Nil(t, err)
}

func TestCloudVpc_getNodeCountInEachZone(t *testing.T) {
	nodes := []*v1.Node{mockNode1, mockNode2, mockNode1}
	result := mockCloud.getNodeCountInEachZone(nodes)
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result["zoneA"], 2)
	assert.Equal(t, result["zoneB"], 1)
}

func TestCloudVpc_getLoadBalancerCountInEachZone(t *testing.T) {
	lbA := &VpcLoadBalancer{Subnets: []VpcObjectReference{{ID: "subnetA"}}}
	lbB := &VpcLoadBalancer{Subnets: []VpcObjectReference{{ID: "subnetA"}, {ID: "subnetB"}}}
	lbC := &VpcLoadBalancer{Subnets: []VpcObjectReference{{ID: "subnetA"}, {ID: "subnetB"}, {ID: "subnetC"}}}
	vpcSubnets := []*VpcSubnet{
		{ID: "subnetA", Zone: "zoneA"},
		{ID: "subnetB", Zone: "zoneB"},
		{ID: "subnetC", Zone: "zoneC"},
	}
	lbs := []*VpcLoadBalancer{lbA, lbB, lbC}
	result := mockCloud.getLoadBalancerCountInEachZone(lbs, vpcSubnets)
	assert.Equal(t, len(result), 3)
	assert.Equal(t, result["zoneA"], 3)
	assert.Equal(t, result["zoneB"], 2)
	assert.Equal(t, result["zoneC"], 1)
}

func TestCloudVpc_GetNodeIDs(t *testing.T) {
	nodes := []*v1.Node{mockNode1, mockNode2, mockNode3}
	c := CloudVpc{}
	nodeIDs := c.getNodeIDs(nodes)
	assert.Equal(t, len(nodeIDs), 2)
	assert.Equal(t, nodeIDs[0], mockNode1.Name)
	assert.Equal(t, nodeIDs[1], mockNode2.Name)
}

func TestCloudVpc_GetNodeInteralIP(t *testing.T) {
	c := CloudVpc{}
	internalIP := c.getNodeInternalIP(mockNode1)
	assert.Equal(t, "192.168.1.1", internalIP)

	internalIP = c.getNodeInternalIP(mockNode2)
	assert.Equal(t, "192.168.2.2", internalIP)

	internalIP = c.getNodeInternalIP(mockNode3)
	assert.Equal(t, "", internalIP)

	internalIP = c.getNodeInternalIP(mockNode4)
	assert.Equal(t, "", internalIP)
}

func TestCloudVpc_GetPoolMemberTargets(t *testing.T) {
	members := []*VpcLoadBalancerPoolMember{{TargetIPAddress: "192.168.1.1", TargetInstanceID: "1234-56-7890"}}
	result := mockCloud.getPoolMemberTargets(members)
	assert.Equal(t, len(result), 1)
	assert.Equal(t, result[0], "192.168.1.1")
}

func TestCloudVpc_getServiceEndpointNodeCounts(t *testing.T) {
	// No endpoint found for the service
	mockService := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "1234"}}
	nodesFound, err := mockCloud.getServiceEndpointNodeCounts(mockService)
	assert.Empty(t, nodesFound)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get default/echo-server endpoints")

	// Endpoint defined for the service, no backend pods
	endpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	mockCloud.KubeClient = fake.NewSimpleClientset(endpoints)
	nodesFound, err = mockCloud.getServiceEndpointNodeCounts(mockService)
	assert.Empty(t, nodesFound)
	assert.Nil(t, err)

	// Endpoint defined for the service, multiple backend pods on multiple nodes
	node1 := mockNode1.Name
	node2 := mockNode2.Name
	endpoints = &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"},
		Subsets: []v1.EndpointSubset{{Addresses: []v1.EndpointAddress{{NodeName: &node1}, {NodeName: &node2}, {NodeName: &node1}}}}}
	mockCloud.KubeClient = fake.NewSimpleClientset(endpoints)
	nodesFound, err = mockCloud.getServiceEndpointNodeCounts(mockService)
	assert.Nil(t, err)
	assert.Equal(t, len(nodesFound), 2)
	assert.Equal(t, nodesFound[node1], 2)
	assert.Equal(t, nodesFound[node2], 1)
	mockCloud.KubeClient = fake.NewSimpleClientset()
}

func TestCloudVpc_getServiceEndpointZoneCounts(t *testing.T) {
	// No endpoint found for the service
	mockService := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "1234"}}
	mockNodes := []*v1.Node{mockNode1, mockNode2}
	zonesFound, err := mockCloud.getServiceEndpointZoneCounts(mockService, mockNodes)
	assert.Empty(t, zonesFound)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get default/echo-server endpoints")

	// Endpoint defined for the service, no backend pods
	endpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	mockCloud.KubeClient = fake.NewSimpleClientset(endpoints)
	zonesFound, err = mockCloud.getServiceEndpointZoneCounts(mockService, mockNodes)
	assert.Empty(t, zonesFound)
	assert.Nil(t, err)

	// Endpoint defined for the service, multiple backend pods on multiple nodes
	node1 := mockNode1.Name
	node2 := mockNode2.Name
	endpoints = &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"},
		Subsets: []v1.EndpointSubset{{Addresses: []v1.EndpointAddress{{NodeName: &node1}, {NodeName: &node2}}}}}
	mockCloud.KubeClient = fake.NewSimpleClientset(endpoints)
	zonesFound, err = mockCloud.getServiceEndpointZoneCounts(mockService, mockNodes)
	assert.Nil(t, err)
	assert.Equal(t, len(zonesFound), 2)
	assert.Equal(t, zonesFound["zoneA"], 1)
	assert.Equal(t, zonesFound["zoneB"], 1)
	mockCloud.KubeClient = fake.NewSimpleClientset()
}

func TestCloudVpc_GetServiceNodeSelectorFilter(t *testing.T) {
	// No annotation on the service. Output should be ""
	mockService := &v1.Service{}
	filterLabel, filterValue := mockCloud.getServiceNodeSelectorFilter(mockService)
	assert.Equal(t, filterLabel, "")
	assert.Equal(t, filterValue, "")

	// Invalid annotation on the service. Output should be ""
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationNodeSelector: "invalid"}
	filterLabel, filterValue = mockCloud.getServiceNodeSelectorFilter(mockService)
	assert.Equal(t, filterLabel, "")
	assert.Equal(t, filterValue, "")

	// Invalid key in the annotation on the service.  Output should be ""
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationNodeSelector: "beta.kubernetes.io/os=linux"}
	filterLabel, filterValue = mockCloud.getServiceNodeSelectorFilter(mockService)
	assert.Equal(t, filterLabel, "")
	assert.Equal(t, filterValue, "")

	// Valid key in the annotation on the service.  Output should match the annotation value
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationNodeSelector: "node.kubernetes.io/instance-type=cx2.2x4"}
	filterLabel, filterValue = mockCloud.getServiceNodeSelectorFilter(mockService)
	assert.Equal(t, filterLabel, "node.kubernetes.io/instance-type")
	assert.Equal(t, filterValue, "cx2.2x4")
}

func TestCloudVpc_GetServiceMemberQuota(t *testing.T) {
	// No annotation on the service. Return the default quota value
	mockService := &v1.Service{}
	quota, err := mockCloud.getServiceMemberQuota(mockService)
	assert.Equal(t, quota, defaultPoolMemberQuota)
	assert.Nil(t, err)

	// Annotation set to disale quota checks
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "disable"}
	quota, err = mockCloud.getServiceMemberQuota(mockService)
	assert.Equal(t, quota, 0)
	assert.Nil(t, err)

	// Invalid annotation on the service
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "invalid"}
	quota, err = mockCloud.getServiceMemberQuota(mockService)
	assert.Equal(t, quota, -1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "is not set to a valid value")

	// Valid quota specified in the annotation on the service
	mockService.ObjectMeta.Annotations = map[string]string{serviceAnnotationMemberQuota: "100"}
	quota, err = mockCloud.getServiceMemberQuota(mockService)
	assert.Equal(t, quota, 100)
	assert.Nil(t, err)
}

func TestCloudVpc_getServicePoolNames(t *testing.T) {
	c := &CloudVpc{}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default",
			Annotations: map[string]string{}},
		Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{Protocol: v1.ProtocolTCP, Port: 80, NodePort: 30123}}},
	}
	// getPoolNamesForService success
	poolNames, err := c.getServicePoolNames(service)
	assert.Nil(t, err)
	assert.Equal(t, len(poolNames), 1)
	assert.Equal(t, poolNames[0], "tcp-80-30123")
}

func TestCloudVpc_getSubnetIDs(t *testing.T) {
	subnets := []*VpcSubnet{{ID: "subnet1"}, {ID: "subnet2"}}
	result := mockCloud.getSubnetIDs(subnets)
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result[0], "subnet1")
	assert.Equal(t, result[1], "subnet2")
}

func TestCloudVpc_getZonesContainingSubnets(t *testing.T) {
	subnets := []*VpcSubnet{{ID: "subnet1", Zone: "zoneA"}, {ID: "subnet2", Zone: "zoneB"}, {ID: "subnet3", Zone: "zoneA"}}
	result := mockCloud.getZonesContainingSubnets(subnets)
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result[0], "zoneA")
	assert.Equal(t, result[1], "zoneB")
}

func TestCloudVpc_IsServicePublic(t *testing.T) {
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	result := mockCloud.isServicePublic(service)
	assert.Equal(t, result, true)

	service.ObjectMeta.Annotations = map[string]string{serviceAnnotationIPType: servicePrivateLB}
	result = mockCloud.isServicePublic(service)
	assert.Equal(t, result, false)
}

func TestCloudVpc_IsVpcConfigStoredInSecret(t *testing.T) {
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: VpcSecretFileName, Namespace: VpcSecretNamespace}}
	result := mockCloud.IsVpcConfigStoredInSecret(secret)
	assert.Equal(t, result, true)
	secret = &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"}}
	result = mockCloud.IsVpcConfigStoredInSecret(secret)
	assert.Equal(t, result, false)
}

func TestCloudVpc_selectSingleZoneForSubnetAndNodes(t *testing.T) {
	// selectSingleZoneForSubnetAndNodes, 2 zones passed in, only nodes in one of the zones
	nodes := []*v1.Node{mockNode1, mockNode2} // Nodes are in zoneA & zoneB
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	subnetZones := []string{"zoneA", "zoneC"}
	vpcSubnets := []*VpcSubnet{{ID: "subnetA", Zone: "zoneA"}, {ID: "subnetC", Zone: "zoneC"}}
	retSubnets, retNodes, err := mockCloud.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, err)
	assert.Equal(t, len(retSubnets), 1)
	assert.Equal(t, retSubnets[0], "subnetA")
	assert.Equal(t, len(retNodes), 1)
	assert.Equal(t, retNodes[0].Name, "192.168.1.1")

	// selectSingleZoneForSubnetAndNodes, 2 zones passed in, no nodes in any of the zones
	nodes = []*v1.Node{mockNode1} // Node1 is in zoneA
	subnetZones = []string{"zoneC", "zoneD"}
	retSubnets, retNodes, err = mockCloud.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, err)
	assert.Equal(t, len(retSubnets), 1)
	assert.Equal(t, retSubnets[0], "subnetC")
	assert.Equal(t, len(retNodes), 0)

	// selectSingleZoneForSubnetAndNodes, failed to get endpoints for the service, ExternalTrafficPolicy: Local
	nodes = []*v1.Node{mockNode1, mockNode2} // Nodes are in zoneA & zoneB
	subnetZones = []string{"zoneA", "zoneB"}
	vpcSubnets = []*VpcSubnet{{ID: "subnetA", Zone: "zoneA"}, {ID: "subnetB", Zone: "zoneB"}}
	service.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	retSubnets, retNodes, err = mockCloud.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, retSubnets)
	assert.Nil(t, retNodes)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed to get default/echo-server endpoints")

	// selectSingleZoneForSubnetAndNodes, failed to get endpoints for the service, ExternalTrafficPolicy: Local
	node1 := mockNode1.Name
	endpoints := &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"},
		Subsets: []v1.EndpointSubset{{Addresses: []v1.EndpointAddress{{NodeName: &node1}}}}}
	mockCloud.KubeClient = fake.NewSimpleClientset(endpoints)
	retSubnets, retNodes, err = mockCloud.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, err)
	assert.Equal(t, len(retSubnets), 1)
	assert.Equal(t, retSubnets[0], "subnetA")
	assert.Equal(t, len(retNodes), 1)
	assert.Equal(t, retNodes[0].Name, "192.168.1.1")
	service.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster

	// selectSingleZoneForSubnetAndNodes, failed to get list of load balancers
	sdk, _ := NewVpcSdkFake()
	c := &CloudVpc{
		Config:     ConfigVpc{ClusterID: "clusterID"},
		Sdk:        sdk,
		KubeClient: fake.NewSimpleClientset(),
	}
	nodes = []*v1.Node{mockNode1, mockNode2} // Nodes are in zoneA & zoneB
	subnetZones = []string{"zoneA", "zoneB"}
	vpcSubnets = []*VpcSubnet{{ID: "subnetA", Zone: "zoneA"}, {ID: "subnetB", Zone: "zoneB"}}
	c.SetFakeSdkError("ListLoadBalancers")
	retSubnets, retNodes, err = c.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, retSubnets)
	assert.Nil(t, retNodes)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "ListLoadBalancers failed")
	c.ClearFakeSdkError("ListLoadBalancers")

	// selectSingleZoneForSubnetAndNodes, retrieved list of load balancers
	retSubnets, retNodes, err = c.selectSingleZoneForSubnetAndNodes(service, vpcSubnets, subnetZones, nodes)
	assert.Nil(t, err)
	assert.Equal(t, len(retSubnets), 1)
	assert.Equal(t, len(retNodes), 1)
}

func TestCloudVpc_selectSubnetZoneForNLB(t *testing.T) {
	lbZones := map[string]int{"zoneA": 1}
	nodeZones := map[string]int{"zoneA": 1, "zoneB": 2, "zoneC": 3}
	// "zoneB" - selected because fewer NLBs in zoneB
	result := mockCloud.selectSubnetZoneForNLB([]string{"zoneA", "zoneB"}, lbZones, nodeZones)
	assert.Equal(t, len(result), 1)
	assert.Equal(t, result[0], "zoneB")

	// "zoneC" - selected because: no NLBs and the most nodes
	result = mockCloud.selectSubnetZoneForNLB([]string{"zoneA", "zoneB", "zoneC"}, lbZones, nodeZones)
	assert.Equal(t, len(result), 1)
	assert.Equal(t, result[0], "zoneC")
}

func TestCloudVpc_ValidateClusterSubnetIDs(t *testing.T) {
	clusterSubnets := []string{"subnetID"}
	vpcSubnets := []*VpcSubnet{{ID: "subnetID"}}

	// validateClusterSubnetIDs, success
	foundSubnets, err := mockCloud.validateClusterSubnetIDs(clusterSubnets, vpcSubnets)
	assert.Equal(t, len(foundSubnets), 1)
	assert.Nil(t, err)

	// validateClusterSubnetIDs failed, invalid subnet ID
	clusterSubnets = []string{"invalid subnet"}
	foundSubnets, err = mockCloud.validateClusterSubnetIDs(clusterSubnets, vpcSubnets)
	assert.Nil(t, foundSubnets)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid VPC subnet")

	// validateClusterSubnetIDs failed, multiple subnets across different VPCs
	clusterSubnets = []string{"subnet1", "subnet2"}
	vpcSubnets = []*VpcSubnet{
		{ID: "subnet1", Vpc: VpcObjectReference{ID: "vpc1"}},
		{ID: "subnet2", Vpc: VpcObjectReference{ID: "vpc2"}},
	}
	foundSubnets, err = mockCloud.validateClusterSubnetIDs(clusterSubnets, vpcSubnets)
	assert.Nil(t, foundSubnets)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "subnets in different VPCs")
}

func TestCloudVpc_validateService(t *testing.T) {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default",
			Annotations: map[string]string{serviceAnnotationEnableFeatures: ""}},
		Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{Protocol: v1.ProtocolTCP, Port: 80}}},
	}
	// validateService, only TCP protocol is supported
	service.Spec.Ports[0].Protocol = v1.ProtocolUDP
	options, err := mockCloud.validateService(service)
	assert.Empty(t, options)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Only TCP is supported")

	// validateService, other options passed on through
	service.ObjectMeta.Annotations[serviceAnnotationEnableFeatures] = "generic-option"
	service.Spec.Ports[0].Protocol = v1.ProtocolTCP
	options, err = mockCloud.validateService(service)
	assert.Equal(t, options, "generic-option")
	assert.Nil(t, err)
}

func TestCloudVpc_ValidateServiceSubnets(t *testing.T) {
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	vpcSubnets := []*VpcSubnet{{ID: "subnetID", Name: "subnetName", Ipv4CidrBlock: "10.240.0.0/24", Vpc: VpcObjectReference{ID: "vpcID"}}}

	// validateServiceSubnets, success
	subnetIDs, err := mockCloud.validateServiceSubnets(service, "subnetID", "vpcID", vpcSubnets)
	assert.Equal(t, len(subnetIDs), 1)
	assert.Nil(t, err)

	// validateServiceSubnets failed, invalid subnet in the service annotation
	subnetIDs, err = mockCloud.validateServiceSubnets(service, "invalid subnet", "vpcID", vpcSubnets)
	assert.Nil(t, subnetIDs)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid VPC subnet")

	// validateServiceSubnets failed, service subnet is in a different VPC
	subnetIDs, err = mockCloud.validateServiceSubnets(service, "subnetID", "vpc2", vpcSubnets)
	assert.Nil(t, subnetIDs)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "located in a different VPC")

	// validateServiceSubnets, success, subnetID, subnetName and CIDR all passed in for the same subnet
	subnetIDs, err = mockCloud.validateServiceSubnets(service, "subnetID,subnetName,10.240.0.0/24", "vpcID", vpcSubnets)
	assert.Equal(t, len(subnetIDs), 1)
	assert.Equal(t, subnetIDs[0], "subnetID")
	assert.Nil(t, err)
}

func TestCloudVpc_ValidateServiceSubnetsNotUpdated(t *testing.T) {
	lb := &VpcLoadBalancer{Subnets: []VpcObjectReference{{ID: "subnetID"}}}
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "echo-server", Namespace: "default",
		Annotations: map[string]string{}},
	}
	vpcSubnets := []*VpcSubnet{{ID: "subnetID"}, {ID: "subnetID2"}}

	// validateServiceSubnetsNotUpdated, success - annotation not set
	err := mockCloud.validateServiceSubnetsNotUpdated(service, lb, vpcSubnets)
	assert.Nil(t, err)

	// validateServiceSubnetsNotUpdated, success - no change in annotation
	service.ObjectMeta.Annotations[serviceAnnotationSubnets] = "subnetID"
	err = mockCloud.validateServiceSubnetsNotUpdated(service, lb, vpcSubnets)
	assert.Nil(t, err)

	// validateServiceSubnetsNotUpdated, Failed, diff subnet specified
	service.ObjectMeta.Annotations[serviceAnnotationSubnets] = "subnetID2"
	err = mockCloud.validateServiceSubnetsNotUpdated(service, lb, vpcSubnets)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "setting can not be changed")
}

func TestCloudVpc_ValidateServiceTypeNotUpdated(t *testing.T) {
	lb := &VpcLoadBalancer{IsPublic: true}
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "echo-server", Namespace: "default",
		Annotations: map[string]string{}},
	}

	// validateServiceTypeNotUpdated, success - annotation not set
	err := mockCloud.validateServiceTypeNotUpdated(service, lb)
	assert.Nil(t, err)

	// validateServiceTypeNotUpdated, success - lb public, service private
	service.ObjectMeta.Annotations[serviceAnnotationIPType] = servicePrivateLB
	err = mockCloud.validateServiceTypeNotUpdated(service, lb)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "setting can not be changed")

	// validateServiceTypeNotUpdated, success - lb private, service public
	lb.IsPublic = false
	service.ObjectMeta.Annotations[serviceAnnotationIPType] = servicePublicLB
	err = mockCloud.validateServiceTypeNotUpdated(service, lb)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "setting can not be changed")
	lb.IsPublic = true
}

func TestCloudVpc_ValidateServiceZone(t *testing.T) {
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default"}}
	vpcSubnets := []*VpcSubnet{{ID: "subnetID", Zone: "zoneA"}}

	// validateServiceZone, success
	subnetIDs, err := mockCloud.validateServiceZone(service, "zoneA", vpcSubnets)
	assert.Equal(t, len(subnetIDs), 1)
	assert.Nil(t, err)

	// validateServiceZone failed, no cluster subnets in that zone
	subnetIDs, err = mockCloud.validateServiceZone(service, "zoneX", vpcSubnets)
	assert.Nil(t, subnetIDs)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no cluster subnets in that zone")
}

func TestCloudVpc_validateServiceZoneNotUpdated(t *testing.T) {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "echo-server", Namespace: "default",
			Annotations: map[string]string{serviceAnnotationZone: "zoneA"}}}
	lb := &VpcLoadBalancer{ProfileFamily: "network"}

	// validateServiceZoneNotUpdated, no subnets on NLB
	err := mockCloud.validateServiceZoneNotUpdated(service, lb, []string{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "created with no subnets")

	// validateServiceZoneNotUpdated, 2 subnets on NLB
	err = mockCloud.validateServiceZoneNotUpdated(service, lb, []string{"zoneA", "zoneB"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "created in zones [zoneA zoneB]")

	// validateServiceZoneNotUpdated, service zone and LB zone different
	err = mockCloud.validateServiceZoneNotUpdated(service, lb, []string{"zoneB"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "setting can not be changed")

	// validateServiceZoneNotUpdated, success - service zone and LB zone same
	err = mockCloud.validateServiceZoneNotUpdated(service, lb, []string{"zoneA"})
	assert.Nil(t, err)
}
