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
	"context"
	"testing"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpcctl"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	cluster     = "clusterID"
	clusterName = "clusterName"
)

func TestCloud_InitCloudVpc(t *testing.T) {
	c := Cloud{Config: &CloudConfig{Prov: Provider{ClusterID: cluster}}, KubeClient: fake.NewSimpleClientset()}
	v, err := c.InitCloudVpc(true)
	assert.Nil(t, v)
	assert.NotNil(t, err)
}

func TestCloud_isProviderVpc(t *testing.T) {
	c := Cloud{Config: &CloudConfig{Prov: Provider{ClusterID: cluster}}}
	result := c.isProviderVpc()
	assert.False(t, result)
	c.Config.Prov.ProviderType = vpcctl.VpcProviderTypeGen2
	result = c.isProviderVpc()
	assert.True(t, result)
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
		AccountID:                "accountID",
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

	// Successfully return ConfigVpc
	c.Config.Prov.G2Credentials = ""
	config, err = c.NewConfigVpc(true)
	assert.NotNil(t, config)
	assert.Nil(t, err)
	assert.Equal(t, config.AccountID, "accountID")
	assert.Equal(t, config.APIKeySecret, "")
	assert.Equal(t, config.ClusterID, "clusterID")
	assert.Equal(t, config.EnablePrivate, true)
	assert.Equal(t, config.ProviderType, "g2")
	assert.Equal(t, config.Region, "us-south")
	assert.Equal(t, config.ResourceGroupName, "Default")
	assert.Equal(t, config.SubnetNames, "subnet1,subnet2,subnet3")
	assert.Equal(t, config.WorkerAccountID, "accountID")
	assert.Equal(t, config.VpcName, "vpc")
}

func TestCloud_VpcEnsureLoadBalancer(t *testing.T) {
	s, _ := vpcctl.NewVpcSdkFake()
	c := &vpcctl.CloudVpc{
		KubeClient: fake.NewSimpleClientset(),
		Config:     &vpcctl.ConfigVpc{ClusterID: "clusterID", ProviderType: vpcctl.VpcProviderTypeFake},
		Sdk:        s}
	cloud := Cloud{
		KubeClient: fake.NewSimpleClientset(),
		Config:     &CloudConfig{Prov: Provider{ClusterID: "clusterID", ProviderType: vpcctl.VpcProviderTypeGen2}},
		Recorder:   NewCloudEventRecorderV1("ibm", fake.NewSimpleClientset().CoreV1().Events("")),
		Vpc:        c,
	}
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.0.1", Labels: map[string]string{}}}

	// VpcEnsureLoadBalancer failed, failed to initialize VPC env
	cloud.Vpc = nil
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	status, err := cloud.VpcEnsureLoadBalancer(context.Background(), clusterName, service, []*v1.Node{node})
	assert.Nil(t, status)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed initializing VPC")
	cloud.Vpc = c

	// VpcEnsureLoadBalancer failed, failed to get create LB, no available nodes
	cloud.Config.Prov.ProviderType = vpcctl.VpcProviderTypeFake
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "NotFound"}}
	status, err = cloud.VpcEnsureLoadBalancer(context.Background(), clusterName, service, []*v1.Node{})
	assert.Nil(t, status)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "There are no available nodes for LoadBalancer")

	// VpcEnsureLoadBalancer successful, existing LB was updated
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	status, err = cloud.VpcEnsureLoadBalancer(context.Background(), clusterName, service, []*v1.Node{node})
	assert.NotNil(t, status)
	assert.Nil(t, err)
	assert.Equal(t, status.Ingress[0].Hostname, "lb.ibm.com")
}

func TestCloud_VpcEnsureLoadBalancerDeleted(t *testing.T) {
	s, _ := vpcctl.NewVpcSdkFake()
	c := &vpcctl.CloudVpc{Sdk: s}
	cloud := Cloud{
		KubeClient: fake.NewSimpleClientset(),
		Config:     &CloudConfig{Prov: Provider{ClusterID: "clusterID"}},
		Recorder:   NewCloudEventRecorderV1("ibm", fake.NewSimpleClientset().CoreV1().Events("")),
		Vpc:        c,
	}

	// VpcEnsureLoadBalancerDeleted failed, failed to initialize VPC env
	cloud.Vpc = nil
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err := cloud.VpcEnsureLoadBalancerDeleted(context.Background(), clusterName, service)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed initializing VPC")
	cloud.Vpc = c

	// VpcEnsureLoadBalancerDeleted failed, failed to delete the LB
	c.SetFakeSdkError("DeleteLoadBalancer")
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err = cloud.VpcEnsureLoadBalancerDeleted(context.Background(), clusterName, service)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed deleting LoadBalancer")
	c.ClearFakeSdkError("DeleteLoadBalancer")

	// VpcEnsureLoadBalancerDeleted successful, existing LB was deleted
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err = cloud.VpcEnsureLoadBalancerDeleted(context.Background(), clusterName, service)
	assert.Nil(t, err)
}

func TestCloud_VpcGetLoadBalancer(t *testing.T) {
	s, _ := vpcctl.NewVpcSdkFake()
	c := &vpcctl.CloudVpc{Sdk: s}
	cloud := Cloud{
		Config:   &CloudConfig{Prov: Provider{ClusterID: "clusterID"}},
		Recorder: NewCloudEventRecorderV1("ibm", fake.NewSimpleClientset().CoreV1().Events("")),
		Vpc:      c,
	}

	// VpcGetLoadBalancer failed, failed to initialize VPC env
	cloud.KubeClient = fake.NewSimpleClientset()
	cloud.Vpc = nil
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	status, exist, err := cloud.VpcGetLoadBalancer(context.Background(), clusterName, service)
	assert.Nil(t, status)
	assert.False(t, exist)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed initializing VPC")
	cloud.Vpc = c

	// VpcGetLoadBalancer failed, failed to get find the LB
	c.SetFakeSdkError("FindLoadBalancer")
	c.SetFakeSdkError("ListLoadBalancers")
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	status, exist, err = cloud.VpcGetLoadBalancer(context.Background(), clusterName, service)
	assert.Nil(t, status)
	assert.False(t, exist)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed getting LoadBalancer")
	c.ClearFakeSdkError("FindLoadBalancer")
	c.ClearFakeSdkError("ListLoadBalancers")

	// VpcGetLoadBalancer successful, LB is ready
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	status, exist, err = cloud.VpcGetLoadBalancer(context.Background(), clusterName, service)
	assert.NotNil(t, status)
	assert.Equal(t, status.Ingress[0].Hostname, "lb.ibm.com")
	assert.True(t, exist)
	assert.Nil(t, err)
}

func TestCloud_VpcGetLoadBalancerName(t *testing.T) {
	clusterID := "12345678901234567890"
	c := Cloud{Config: &CloudConfig{Prov: Provider{ClusterID: clusterID}}}
	kubeService := &v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "echo-server", Namespace: "default", UID: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"}}
	lbName := vpcctl.VpcLbNamePrefix + "-" + clusterID + "-" + string(kubeService.UID)
	lbName = lbName[:63]
	result := c.vpcGetLoadBalancerName(kubeService)
	assert.Equal(t, result, lbName)
}

func TestCloud_VpcHandleSecretAdd(t *testing.T) {
	// VPC environment not initialized
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"}}
	c := Cloud{}
	c.VpcHandleSecretAdd(secret)
	assert.Nil(t, c.Vpc)
	// VPC environment initialized, secret does not contain any VPC data
	c.Vpc = &vpcctl.CloudVpc{}
	c.VpcHandleSecretAdd(secret)
	assert.NotNil(t, c.Vpc)
}

func TestCloud_VpcHandleSecretDelete(t *testing.T) {
	// VPC environment not initialized
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"}}
	c := Cloud{}
	c.VpcHandleSecretDelete(secret)
	assert.Nil(t, c.Vpc)
	// VPC environment initialized, secret does not contain any VPC data
	c.Vpc = &vpcctl.CloudVpc{}
	c.VpcHandleSecretDelete(secret)
	assert.NotNil(t, c.Vpc)
}

func TestCloud_VpcHandleSecretUpdate(t *testing.T) {
	// VPC environment not initialized
	secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"}}
	c := Cloud{}
	c.VpcHandleSecretUpdate(nil, secret)
	assert.Nil(t, c.Vpc)
	// VPC environment initialized, secret does not contain any VPC data
	c.Vpc = &vpcctl.CloudVpc{}
	c.VpcHandleSecretUpdate(nil, secret)
	assert.NotNil(t, c.Vpc)
}

func TestCloud_VpcMonitorLoadBalancers(t *testing.T) {
	s, _ := vpcctl.NewVpcSdkFake()
	c := &vpcctl.CloudVpc{
		Sdk:    s,
		Config: &vpcctl.ConfigVpc{ClusterID: "clusterID"},
	}
	cloud := Cloud{
		Config:   &CloudConfig{Prov: Provider{ClusterID: "clusterID"}},
		Recorder: NewCloudEventRecorderV1("ibm", fake.NewSimpleClientset().CoreV1().Events("")),
		Vpc:      c,
	}
	serviceNodePort := v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nodePort", Namespace: "default", UID: "NodePort"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeNodePort}}
	serviceNotFound := v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notFound", Namespace: "default", UID: "NotFound"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	serviceNotReady := v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "notReady", Namespace: "default", UID: "NotReady"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	serviceReady := v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "Ready", Namespace: "default", UID: "Ready"},
		Spec: v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer}}
	serviceList := &v1.ServiceList{Items: []v1.Service{serviceNodePort}}
	dataMap := map[string]string{}

	// VpcUpdateLoadBalancer failed, failed to initialize VPC env
	cloud.KubeClient = fake.NewSimpleClientset()
	cloud.Vpc = nil
	cloud.VpcMonitorLoadBalancers(serviceList, dataMap)
	cloud.Vpc = c

	// VpcUpdateLoadBalancer failed, service list was not passed in
	cloud.VpcMonitorLoadBalancers(nil, dataMap)

	// VpcUpdateLoadBalancer failed, service list was not passed in
	cloud.VpcMonitorLoadBalancers(&v1.ServiceList{Items: []v1.Service{}}, dataMap)

	// VpcUpdateLoadBalancer success, no existing data
	serviceList = &v1.ServiceList{Items: []v1.Service{serviceNodePort, serviceNotFound, serviceNotReady}}
	cloud.VpcMonitorLoadBalancers(serviceList, dataMap)
	assert.Equal(t, len(dataMap), 2)
	assert.Equal(t, dataMap["NotFound"], vpcLbStatusOfflineNotFound)
	assert.Equal(t, dataMap["NotReady"], vpcLbStatusOfflineCreatePending)

	// VpcUpdateLoadBalancer success, data updated based on current state
	serviceList = &v1.ServiceList{Items: []v1.Service{serviceReady}}
	dataMap = map[string]string{"Ready": vpcLbStatusOfflineCreatePending}
	cloud.VpcMonitorLoadBalancers(serviceList, dataMap)
	assert.Equal(t, len(dataMap), 1)
	assert.Equal(t, dataMap["Ready"], vpcLbStatusOnlineActive)

	// VpcUpdateLoadBalancer success, no change is status
	serviceList = &v1.ServiceList{Items: []v1.Service{serviceNotReady, serviceNotFound}}
	dataMap = map[string]string{"NotReady": vpcLbStatusOfflineCreatePending, "NotFound": vpcLbStatusOfflineNotFound}
	cloud.VpcMonitorLoadBalancers(serviceList, dataMap)
	assert.Equal(t, len(dataMap), 2)
	assert.Equal(t, dataMap["NotReady"], vpcLbStatusOfflineCreatePending)
	assert.Equal(t, dataMap["NotFound"], vpcLbStatusOfflineNotFound)
}

func TestCloud_VpcUpdateLoadBalancer(t *testing.T) {
	s, _ := vpcctl.NewVpcSdkFake()
	c := &vpcctl.CloudVpc{Sdk: s}
	cloud := Cloud{
		Config:   &CloudConfig{Prov: Provider{ClusterID: "clusterID"}},
		Recorder: NewCloudEventRecorderV1("ibm", fake.NewSimpleClientset().CoreV1().Events("")),
		Vpc:      c,
	}
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "192.168.0.1", Labels: map[string]string{}}}

	// VpcUpdateLoadBalancer failed, failed to initialize VPC env
	cloud.KubeClient = fake.NewSimpleClientset()
	cloud.Vpc = nil
	service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err := cloud.VpcUpdateLoadBalancer(context.Background(), clusterName, service, []*v1.Node{node})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Failed initializing VPC")
	cloud.Vpc = c

	// VpcUpdateLoadBalancer failed, failed to update LB, node list is empty
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err = cloud.VpcUpdateLoadBalancer(context.Background(), clusterName, service, []*v1.Node{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "There are no available nodes for LoadBalancer")

	// VpcUpdateLoadBalancer successful, existing LB was updated
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo-server", Namespace: "default", UID: "Ready"}}
	err = cloud.VpcUpdateLoadBalancer(context.Background(), clusterName, service, []*v1.Node{node})
	assert.Nil(t, err)
}
