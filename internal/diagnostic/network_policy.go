package diagnostic

/*
 * Cilium Routing Mode Documentation
 *
 * This file previously contained code to simulate Cilium failures using network policies.
 * That approach has been replaced by using actual Cilium misconfigurations through routing mode settings.
 *
 * Available Cilium routing modes:
 *
 * 1. tunnel (default)
 *    - Creates an overlay network using encapsulation protocols like VXLAN or Geneve
 *    - Pod traffic is encapsulated and tunneled between nodes
 *    - Suitable for most environments without special networking requirements
 *
 * 2. native
 *    - Uses the native routing capability of the underlying network
 *    - No encapsulation overhead
 *    - Requires the underlying network to route pod CIDR ranges
 *    - Better performance than tunnel mode but requires compatible network topology
 *
 * 3. direct
 *    - Direct routing without encapsulation or NAT
 *    - Pods communicate directly with external services without NAT
 *    - Requires the external network to route pod CIDR ranges
 *    - Best performance but has specific networking requirements
 *
 * Using the build_test_k8s.sh script with -r/--routing flag allows selecting these modes:
 *    ./build_test_k8s.sh -r native    # Use native routing mode
 *    ./build_test_k8s.sh -r direct    # Use direct routing mode
 *    ./build_test_k8s.sh              # Uses default tunnel mode
 *
 * The diagnostic tests should detect connectivity issues when an incompatible
 * routing mode is used for the network environment.
 */
