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
	"reflect"
	"testing"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpcctl"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	cluster = "bqcssbbd0bsui62odcdg"
)

func getSecretNotFound() kubernetes.Interface {
	return fake.NewSimpleClientset()
}
func getSecretData(secretData string) kubernetes.Interface {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: vpcctl.VpcSecretFileName, Namespace: vpcctl.VpcSecretNamespace},
		Data:       map[string][]byte{vpcctl.VpcClientDataKey: []byte(secretData)},
	}
	return fake.NewSimpleClientset(secret)
}

var gen2Data = `[VPC]
   g2_riaas_endpoint_url = "https://us-south.iaas.cloud.ibm.com:443"
   g2_riaas_endpoint_private_url = "https://private-us-south.iaas.cloud.ibm.com:443"
   g2_resource_group_id = "resourceGroup"
   g2_api_key = "foobar"
   provider_type = "g2"`
var gen2CloudVpc = &vpcctl.CloudVpc{
	Config: vpcctl.ConfigVpc{
		APIKeySecret:     "foobar",
		ClusterID:        cluster,
		EnablePrivate:    true,
		EndpointURL:      "https://private-us-south.iaas.cloud.ibm.com:443/v1",
		ProviderType:     "g2",
		ResourceGroupID:  "resourceGroup",
		TokenExchangeURL: "https://private.iam.cloud.ibm.com/identity/token",
	}}

func TestNewCloudVpc(t *testing.T) {
	type args struct {
		kubeClient            kubernetes.Interface
		clusterID             string
		enablePrivateEndpoint bool
	}
	tests := []struct {
		name    string
		args    args
		want    *vpcctl.CloudVpc
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
			name: "Valid Gen2 secret - private service endpoint",
			args: args{kubeClient: getSecretData(gen2Data), clusterID: cluster, enablePrivateEndpoint: true},
			want: gen2CloudVpc, wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCloudVpc(tt.args.kubeClient, tt.args.clusterID, tt.args.enablePrivateEndpoint)
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

func TestCloud_InitCloudVpc(t *testing.T) {
	c := Cloud{Config: &CloudConfig{Prov: Provider{ClusterID: cluster}}, KubeClient: getSecretNotFound()}
	err := c.InitCloudVpc(true)
	assert.NotNil(t, err)
	c.KubeClient = getSecretData(gen2Data)
	err = c.InitCloudVpc(true)
	assert.Nil(t, err)
}
