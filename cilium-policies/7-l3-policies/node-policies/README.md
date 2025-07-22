# Node-Based L3 Network Policies

This directory contains Cilium network policies that leverage node information to control network traffic between pods based on their node placement.

## Policies Included

- **l3-node-policy.yaml**: Basic policy for controlling traffic based on node identity
- **node-based-policy-clusterwide.yaml**: ClusterWide policy to enforce node-based traffic rules across the entire cluster
- **node-cidr-policy.yaml**: Policy that uses node CIDR ranges to enforce traffic rules
- **pod-node-name-policy.yaml**: Policy that references pod nodeNames for more granular control
- **traditional-node-selector.yaml**: Example of node selection using traditional kubernetes selectors

