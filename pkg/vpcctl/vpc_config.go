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
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultPoolMemberQuota = 50

	// IAM Token Exchange URLs
	iamPrivateTokenExchangeURL         = "https://private.iam.cloud.ibm.com"      // #nosec G101 IBM Cloud iam prod private URL
	iamPublicTokenExchangeURL          = "https://iam.cloud.ibm.com"              // #nosec G101 IBM Cloud iam prod public URL
	iamStagePrivateTokenExchangeURL    = "https://private.iam.test.cloud.ibm.com" // #nosec G101 IBM Cloud iam stage private URL
	iamStageTestPublicTokenExchangeURL = "https://iam.stage1.bluemix.net"         // #nosec G101 IBM Cloud iam stage public URL

	nodeLabelDedicated  = "dedicated"
	nodeLabelInternalIP = "ibm-cloud.kubernetes.io/internal-ip"
	nodeLabelValueEdge  = "edge"
	nodeLabelZone       = "ibm-cloud.kubernetes.io/zone"

	serviceAnnotationEnableFeatures = "service.kubernetes.io/ibm-load-balancer-cloud-provider-enable-features"
	serviceAnnotationIPType         = "service.kubernetes.io/ibm-load-balancer-cloud-provider-ip-type"
	serviceAnnotationMemberQuota    = "service.kubernetes.io/ibm-load-balancer-cloud-provider-vpc-member-quota"
	serviceAnnotationNodeSelector   = "service.kubernetes.io/ibm-load-balancer-cloud-provider-vpc-node-selector"
	serviceAnnotationSubnets        = "service.kubernetes.io/ibm-load-balancer-cloud-provider-vpc-subnets"
	serviceAnnotationZone           = "service.kubernetes.io/ibm-load-balancer-cloud-provider-zone"
	servicePrivateLB                = "private"
	servicePublicLB                 = "public"

	// VpcEndpointIaaSBaseURL - baseURL for constructing the VPC infrastructure API Endpoint URL
	vpcEndpointIaaSProdURL  = "iaas.cloud.ibm.com"
	vpcEndpointIaaSStageURL = "iaasdev.cloud.ibm.com"

	// VpcProviderTypeFake - Fake SDK interface for VPC
	VpcProviderTypeFake = "fake"
	// VpcProviderTypeGen2 - IKS provider type for VPC Gen2
	VpcProviderTypeGen2 = "g2"
)

var memberNodeLabelsAllowed = [...]string{
	"ibm-cloud.kubernetes.io/internal-ip",
	"ibm-cloud.kubernetes.io/machine-type",
	"ibm-cloud.kubernetes.io/os",
	"ibm-cloud.kubernetes.io/region",
	"ibm-cloud.kubernetes.io/subnet-id",
	"ibm-cloud.kubernetes.io/worker-id",
	"ibm-cloud.kubernetes.io/worker-pool-id",
	"ibm-cloud.kubernetes.io/worker-pool-name",
	"ibm-cloud.kubernetes.io/worker-version",
	"ibm-cloud.kubernetes.io/zone",
	"kubernetes.io/arch",
	"kubernetes.io/hostname",
	"kubernetes.io/os",
	"node.kubernetes.io/instance-type",
	"topology.kubernetes.io/region",
	"topology.kubernetes.io/zone",
}

// VpcLbNamePrefix - Prefix to be used for VPC load balancer
var VpcLbNamePrefix = "kube"

// ConfigVpc is the VPC configuration information
type ConfigVpc struct {
	// Externalized config settings from caller
	APIKeySecret      string
	ClusterID         string
	EnablePrivate     bool
	ProviderType      string
	Region            string
	ResourceGroupName string
	SubnetNames       string
	WorkerAccountID   string // Not used, ignored
	VpcName           string
	// Internal config settings
	endpointURL      string
	resourceGroupID  string
	tokenExchangeURL string
}

// CloudVpc is the main VPC cloud provider implementation.
type CloudVpc struct {
	KubeClient kubernetes.Interface
	Config     *ConfigVpc
	Sdk        CloudVpcSdk
}

// getIamEndpoint - retrieve the correct IAM endpoint for the current config
func (c *ConfigVpc) getIamEndpoint() string {
	if strings.Contains(c.Region, "stage") {
		if c.EnablePrivate {
			return iamStagePrivateTokenExchangeURL
		}
		return iamStageTestPublicTokenExchangeURL
	}
	if c.EnablePrivate {
		return iamPrivateTokenExchangeURL
	}
	return iamPublicTokenExchangeURL
}

// getVpcEndpoint - retrieve the correct VPC endpoint for the current config
func (c *ConfigVpc) getVpcEndpoint() string {
	endpoint := vpcEndpointIaaSProdURL
	if strings.Contains(c.Region, "stage") {
		endpoint = vpcEndpointIaaSStageURL
	}
	if c.EnablePrivate {
		return fmt.Sprintf("https://%s.%s.%s", c.Region, "private", endpoint)
	}
	return fmt.Sprintf("https://%s.%s", c.Region, endpoint)
}

// initialize - initialize VPC config fields that were not set by the cloud provider
func (c *ConfigVpc) initialize() error {
	// Validate the config values that were passed in
	err := c.validate()
	if err != nil {
		return err
	}
	if c.ProviderType == VpcProviderTypeFake {
		return nil
	}
	// Determine the VPC endpoint URL
	c.endpointURL = c.getVpcEndpoint()
	c.endpointURL += "/v1"

	// Determine the token exchange URL
	c.tokenExchangeURL = c.getIamEndpoint()
	c.tokenExchangeURL += "/identity/token"
	return nil
}

// validate - verify the config data stored in the ConfigVpc object
func (c *ConfigVpc) validate() error {
	// Check the fields in the config
	switch {
	case c.ClusterID == "":
		return fmt.Errorf("Missing required cloud configuration setting: clusterID")
	case c.ProviderType == VpcProviderTypeFake:
		return nil
	case c.ProviderType != VpcProviderTypeGen2:
		return fmt.Errorf("Invalid cloud configuration setting for cluster-default-provider: %s", c.ProviderType)
	case c.APIKeySecret == "":
		return fmt.Errorf("Missing required cloud configuration setting: g2Credentials")
	case c.Region == "":
		return fmt.Errorf("Missing required cloud configuration setting: region")
	case c.ResourceGroupName == "":
		return fmt.Errorf("Missing required cloud configuration setting: g2ResourceGroupName")
	case c.SubnetNames == "":
		return fmt.Errorf("Missing required cloud configuration setting: g2VpcSubnetNames")
	case c.VpcName == "":
		return fmt.Errorf("Missing required cloud configuration setting: g2VpcName")
	}
	// Validation passed
	return nil
}

// NewCloudVpc - create new CloudVpc object based on the config data that was passed in
func NewCloudVpc(kubeClient kubernetes.Interface, config *ConfigVpc) (*CloudVpc, error) {
	if config == nil {
		return nil, fmt.Errorf("Missing cloud configuration")
	}
	c := &CloudVpc{KubeClient: kubeClient, Config: config}
	err := c.Config.initialize()
	if err != nil {
		return nil, err
	}
	c.Sdk, err = NewCloudVpcSdk(c.Config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// filterNodesByEdgeLabel - extract only the edge nodes if there any any -or- return all nodes
func (c *CloudVpc) filterNodesByEdgeLabel(nodes []*v1.Node) []*v1.Node {
	edgeNodes := c.findNodesMatchingLabelValue(nodes, nodeLabelDedicated, nodeLabelValueEdge)
	if len(edgeNodes) == 0 {
		return nodes
	}
	return edgeNodes
}

// filterNodesByServiceMemberQuota - limit the nodes we select based on the current quota from service annotation
func (c *CloudVpc) filterNodesByServiceMemberQuota(desiredNodes, existingNodes []string, service *v1.Service) ([]string, error) {
	// If externalTrafficPolicy:Local is enabled on the service, then simply return the desired list.  No filtering will be done
	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		return desiredNodes, nil
	}

	// Determine the quota we should use. If annotation is not set, use default. If the annotation is not properly formatted, return error
	quota, err := c.getServiceMemberQuota(service)
	if err != nil {
		return nil, err
	}
	// If the quota is disabled -OR- if the desired node count is <= to the quota, return the desired nodes
	if quota == 0 || len(desiredNodes) <= quota {
		return desiredNodes, nil
	}

	// Now we need to merge the desired and existing node lists into a combined list that is less than the quota
	finalNodes := []string{}
	remainingNodes := []string{}

	// On the CreateLB path, we won't have any existing nodes for the LB so the following logic will be skipped
	// On the UpdateLB path, we want to give preference to the existing nodes on the LB instead of new nodes that were just added to the cluster
	if len(existingNodes) > 0 {
		existingNodeList := " " + strings.Join(existingNodes, " ") + " "
		for _, desiredNode := range desiredNodes {
			if strings.Contains(existingNodeList, " "+desiredNode+" ") {
				finalNodes = append(finalNodes, desiredNode)
				if len(finalNodes) == quota {
					return finalNodes, nil
				}
			} else {
				remainingNodes = append(remainingNodes, desiredNode)
			}
		}
		// Update the desired nodes list to contain only those nodes that have NOT already been moved into final node list
		desiredNodes = remainingNodes
	}

	// Copy over the desired nodes until the quota is reached
	for _, desiredNode := range desiredNodes {
		finalNodes = append(finalNodes, desiredNode)
		if len(finalNodes) == quota {
			break
		}
	}

	// Return list of nodes
	return finalNodes, nil
}

// filterNodesByServiceZone - remove all nodes that don't satisfy service zone annotation
func (c *CloudVpc) filterNodesByServiceZone(nodes []*v1.Node, service *v1.Service) []*v1.Node {
	zone := c.getServiceZone(service)
	if zone != "" {
		return c.findNodesMatchingLabelValue(nodes, nodeLabelZone, zone)
	}
	return nodes
}

// filterSubnetsByVpcName - find all of the subnets in the requested zone
func (c *CloudVpc) filterSubnetsByName(subnets []*VpcSubnet, subnetList string) []*VpcSubnet {
	desiredSubnets := "," + subnetList + ","
	matchingSubnets := []*VpcSubnet{}
	for _, subnet := range subnets {
		if strings.Contains(desiredSubnets, subnet.Name) {
			matchingSubnets = append(matchingSubnets, subnet)
		}
	}
	// Return matching subnets
	return matchingSubnets
}

// filterSubnetsByVpcName - find all of the subnets in the requested zone
func (c *CloudVpc) filterSubnetsByVpcName(subnets []*VpcSubnet, vpcName string) []*VpcSubnet {
	matchingSubnets := []*VpcSubnet{}
	for _, subnet := range subnets {
		if subnet.Vpc.Name == vpcName {
			matchingSubnets = append(matchingSubnets, subnet)
		}
	}
	// Return matching subnets
	return matchingSubnets
}

// findNodesMatchingLabelValue - find all of the nodes that match the requested label and value
func (c *CloudVpc) findNodesMatchingLabelValue(nodes []*v1.Node, filterLabel, filterValue string) []*v1.Node {
	matchingNodes := []*v1.Node{}
	for _, node := range nodes {
		if v, ok := node.Labels[filterLabel]; ok && v == filterValue {
			matchingNodes = append(matchingNodes, node)
		}
	}
	// Return matching nodes
	return matchingNodes
}

// GenerateLoadBalancerName - generate the VPC load balancer name from the cluster ID and Kube service
func (c *CloudVpc) GenerateLoadBalancerName(service *v1.Service) string {
	serviceID := strings.ReplaceAll(string(service.ObjectMeta.UID), "-", "")
	lbName := VpcLbNamePrefix + "-" + c.Config.ClusterID + "-" + serviceID
	// Limit the LB name to 63 characters
	if len(lbName) > 63 {
		lbName = lbName[:63]
	}
	return lbName
}

// getNodeIDs - get the node identifier for each node in the list
func (c *CloudVpc) getNodeIDs(nodeList []*v1.Node) []string {
	nodeIDs := []string{}
	for _, node := range nodeList {
		nodeInternalAddress := c.getNodeInternalIP(node)
		if nodeInternalAddress != "" {
			nodeIDs = append(nodeIDs, nodeInternalAddress)
		}
	}
	return nodeIDs
}

// getNodeInternalIP - get the Internal IP of the node from label or status
func (c *CloudVpc) getNodeInternalIP(node *v1.Node) string {
	nodeInternalAddress := node.Labels[nodeLabelInternalIP]
	if nodeInternalAddress == "" {
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeInternalIP {
				nodeInternalAddress = address.Address
				break
			}
		}
	}
	return nodeInternalAddress
}

// getPoolMemberTargets - get the targets (IP address/Instance ID) for all of the pool members
func (c *CloudVpc) getPoolMemberTargets(members []*VpcLoadBalancerPoolMember) []string {
	memberTargets := []string{}
	for _, member := range members {
		memberTargets = append(memberTargets, member.TargetIPAddress)
	}
	return memberTargets
}

// getServiceEnabledFeatures - retrieve the vpc-subnets annotation
func (c *CloudVpc) getServiceEnabledFeatures(service *v1.Service) string {
	return strings.ToLower(strings.ReplaceAll(service.ObjectMeta.Annotations[serviceAnnotationEnableFeatures], " ", ""))
}

// getServiceHealthCheckNodePort - retrieve the health check node port for the service
func (c *CloudVpc) getServiceHealthCheckNodePort(service *v1.Service) int {
	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		return int(service.Spec.HealthCheckNodePort)
	}
	return 0
}

// getServiceNodeSelectorFilter - retrieve the service annotation used to filter the backend worker nodes
func (c *CloudVpc) getServiceNodeSelectorFilter(service *v1.Service) (string, string) {
	filter := strings.ReplaceAll(service.ObjectMeta.Annotations[serviceAnnotationNodeSelector], " ", "")
	if filter == "" {
		return "", ""
	}
	filterLabelValue := strings.Split(filter, "=")
	if len(filterLabelValue) != 2 {
		return "", ""
	}
	filterLabel := filterLabelValue[0]
	filterValue := filterLabelValue[1]
	for _, allowed := range memberNodeLabelsAllowed {
		if filterLabel == allowed {
			return filterLabel, filterValue
		}
	}
	return "", ""
}

// getServiceMemberQuota - retrieve the service annotation used to filter the backend worker nodes
func (c *CloudVpc) getServiceMemberQuota(service *v1.Service) (int, error) {
	quota := strings.ToLower(service.ObjectMeta.Annotations[serviceAnnotationMemberQuota])
	if quota == "" {
		return defaultPoolMemberQuota, nil
	}
	// If quota checking is disabled, return 0
	if quota == "disable" || quota == "max" {
		return 0, nil
	}
	// Convert quota string to an int
	val, err := strconv.Atoi(quota)
	if err != nil {
		return -1, fmt.Errorf("The annotation %s on service %s/%s is not set to a valid value [%s]",
			serviceAnnotationMemberQuota, service.ObjectMeta.Namespace, service.ObjectMeta.Name, quota)
	}
	// Return result
	return val, nil
}

// getServicePoolNames - get list of pool names for the service ports
func (c *CloudVpc) getServicePoolNames(service *v1.Service) ([]string, error) {
	poolList := []string{}
	if service == nil {
		return poolList, fmt.Errorf("Service not specified")
	}
	for _, kubePort := range service.Spec.Ports {
		poolList = append(poolList, genLoadBalancerPoolName(kubePort))
	}
	return poolList, nil
}

// getServiceSubnets - retrieve the vpc-subnets annotation
func (c *CloudVpc) getServiceSubnets(service *v1.Service) string {
	return strings.ReplaceAll(service.ObjectMeta.Annotations[serviceAnnotationSubnets], " ", "")
}

// getServiceZone - retrieve the zone annotation
func (c *CloudVpc) getServiceZone(service *v1.Service) string {
	return strings.ReplaceAll(service.ObjectMeta.Annotations[serviceAnnotationZone], " ", "")
}

// getSubnetIDs - get the IDs for all of the subnets that were passed in
func (c *CloudVpc) getSubnetIDs(subnets []*VpcSubnet) []string {
	subnetIDs := []string{}
	for _, subnet := range subnets {
		subnetIDs = append(subnetIDs, subnet.ID)
	}
	// Return the IDs of all of the subnets
	return subnetIDs
}

// isServicePortEqualListener - does the specified service port equal the values specified
func (c *CloudVpc) isServicePortEqualListener(kubePort v1.ServicePort, listener *VpcLoadBalancerListener) bool {
	return int(listener.Port) == int(kubePort.Port) &&
		strings.EqualFold(listener.Protocol, string(kubePort.Protocol))
}

// isServicePortEqualPoolName - does the specified service port equal the fields of a pool name
func (c *CloudVpc) isServicePortEqualPoolName(kubePort v1.ServicePort, poolName *VpcPoolNameFields) bool {
	return poolName.Port == int(kubePort.Port) &&
		strings.EqualFold(poolName.Protocol, string(kubePort.Protocol))
}

// isServicePublic - is the Kube service a public load balancer
func (c *CloudVpc) isServicePublic(service *v1.Service) bool {
	value := service.ObjectMeta.Annotations[serviceAnnotationIPType]
	return value == "" || value == servicePublicLB
}

// IsVpcConfigStoredInSecret - does the specified secret contain any VPC related config information
func (c *CloudVpc) IsVpcConfigStoredInSecret(secret *v1.Secret) bool {
	return false
}

// validateService - validate the service and the requested features on the service
func (c *CloudVpc) validateService(service *v1.Service) (string, error) {
	options := c.getServiceEnabledFeatures(service)
	// Only TCP is supported
	for _, kubePort := range service.Spec.Ports {
		if kubePort.Protocol != v1.ProtocolTCP {
			return "", fmt.Errorf("Service %s/%s is a %s load balancer. Only TCP is supported",
				service.ObjectMeta.Namespace, service.ObjectMeta.Name, kubePort.Protocol)
		}
	}
	// All other service annotation options we ignore and just pass through
	return options, nil
}

// Validate the subnets annotation on the service
func (c *CloudVpc) validateServiceSubnets(service *v1.Service, serviceSubnets, vpcID string, vpcSubnets []*VpcSubnet) ([]string, error) {
	desiredSubnetMap := map[string]bool{}
	for _, subnetID := range strings.Split(serviceSubnets, ",") {
		found := false
		for _, subnet := range vpcSubnets {
			if subnetID == subnet.ID {
				if vpcID != subnet.Vpc.ID {
					return nil, fmt.Errorf("The annotation %s on service %s/%s contains VPC subnet %s that is located in a different VPC",
						serviceAnnotationSubnets, service.ObjectMeta.Namespace, service.ObjectMeta.Name, subnetID)
				}
				found = true
				desiredSubnetMap[subnetID] = true
				break
			}
			// Make sure that we only look at subnet names and CIDRs in the current VPC
			if vpcID != subnet.Vpc.ID {
				continue
			}
			// Check to see if the subnet in the service annotation matches the VPC subnet's name or CIDR
			if subnetID == subnet.Name || subnetID == subnet.Ipv4CidrBlock {
				found = true
				desiredSubnetMap[subnet.ID] = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("The annotation %s on service %s/%s contains invalid VPC subnet %s",
				serviceAnnotationSubnets, service.ObjectMeta.Namespace, service.ObjectMeta.Name, subnetID)
		}
	}
	// The user may have specified the same service "value" on the annotation multiple times: ID, name, and CIDR
	// Using a map to hold initail evaluation allows us to easily filter out any repeats
	desiredSubnets := []string{}
	for subnet := range desiredSubnetMap {
		desiredSubnets = append(desiredSubnets, subnet)
	}

	// Return list of VPC subnet IDs
	return desiredSubnets, nil
}

// Validate that the subnets service annotation was not updated
func (c *CloudVpc) validateServiceSubnetsNotUpdated(service *v1.Service, lb *VpcLoadBalancer, vpcSubnets []*VpcSubnet) error {
	// If the annotation is not set, return
	serviceSubnets := c.getServiceSubnets(service)
	if serviceSubnets == "" {
		return nil
	}
	// Translate the subnet service annotation into actual subnet IDs
	vpcID := lb.getVpcID(vpcSubnets)
	requested, err := c.validateServiceSubnets(service, serviceSubnets, vpcID, vpcSubnets)
	if err != nil {
		return err
	}
	// Translate the LB subnet IDs into an array
	actual := []string{}
	for _, subnet := range lb.Subnets {
		actual = append(actual, subnet.ID)
	}
	// Compare the request subnet IDs from the annotation with the actual subnet IDs of the load balancer
	sort.Strings(requested)
	sort.Strings(actual)
	if strings.Join(requested, ",") != strings.Join(actual, ",") {
		return fmt.Errorf("The load balancer was created with subnets %s. This setting can not be changed", strings.Join(actual, ","))
	}
	// No update was detected
	return nil
}

// Validate that the public/private annotation on the service was not updated
func (c *CloudVpc) validateServiceTypeNotUpdated(service *v1.Service, lb *VpcLoadBalancer) error {
	if c.isServicePublic(service) != lb.IsPublic {
		lbType := servicePrivateLB
		if lb.IsPublic {
			lbType = servicePublicLB
		}
		return fmt.Errorf("The load balancer was created as a %s load balancer. This setting can not be changed", lbType)
	}
	return nil
}

// Validate the zone annotation on the service
func (c *CloudVpc) validateServiceZone(service *v1.Service, serviceZone string, vpcSubnets []*VpcSubnet) ([]string, error) {
	clusterSubnets := []string{}
	for _, subnet := range vpcSubnets {
		if serviceZone == subnet.Zone {
			clusterSubnets = append(clusterSubnets, subnet.ID)
		}
	}
	if len(clusterSubnets) == 0 {
		return nil, fmt.Errorf("The annotation %s on service %s/%s contains invalid zone %s. There are no cluster subnets in that zone",
			serviceAnnotationZone, service.ObjectMeta.Namespace, service.ObjectMeta.Name, serviceZone)
	}
	return clusterSubnets, nil
}
