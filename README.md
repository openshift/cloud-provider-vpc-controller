# IBM Cloud: vpcctl

A Kubernetes load balancer provider for IBM Cloud VPC

## Description

This repo contains logic to build the VPCCTL binary that is called by the IBM Cloud Controler Manager to provide VPC load balancers

## IBM Cloud: Kubernetes LoadBalancer options

Only a subset of the IBM Cloud Kubernetes service annotations are supported:

- `service.kubernetes.io/ibm-load-balancer-cloud-provider-ip-type` = "public" | "private"
- `service.kubernetes.io/ibm-load-balancer-cloud-provider-vpc-node-selector`
- `service.kubernetes.io/ibm-load-balancer-cloud-provider-vpc-subnets`
- `service.kubernetes.io/ibm-load-balancer-cloud-provider-zone`

The following node label is supported:

- `dedicated: edge`

See [IBM Cloud documentation](https://cloud.ibm.com/docs/containers?topic=containers-vpc-lbaas) for additional information.

## Contributing
See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidelines.
