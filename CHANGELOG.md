# v2.3.0

* Update base alpine image from 3.9 to 3.12.

# v2.2.0

* Add support for discovering nodes via an SRV record.
* Add support for enabling TLS.

# v2.1.0

* Register with Target Group by IP address instead of instance ID

  When using `--registration-provider=lb`, rather than adding the
  instance ID to the target group it will instead use the instance's
  private IP address. This gets around the issue of asymmetric routing when
  trying to load-balance to its own address.

  See https://aws.amazon.com/premiumsupport/knowledge-center/target-connection-fails-load-balancer/

# v2.0.0

**If upgrading from an older version of etcd-bootstrap this is a breaking change. Please refer to the docs on which command
line arguments are required**

Release v2.0.0 contains a substantial refactor of etcd-bootstrap including:
* Using cobra for command line arguments
* Split out provider from bootstrap logic to allow for better code segregation
* Support mocking for the etcd member client in better test code coverage
* More extensive testing for both the bootstrap and AWS packages
* Replace vendor packages with go modules
* Versioned binaries (can be printed using `./etcd-bootstrap --version`)

Additional functionality:
* Support loadbalancer registration (cannot be used with the DNS registration) which after generating the cluster
    config, adds the etcd instances to a loadbalancer target group if supported by the cloud provider
    (can only be used with the AWS provider).

# v1.2.1

* Update alpine image to fix CVE-2019-5021.
* Update to go version 1.11

# v1.2.0

* Split `start-etcd.sh` into `bootstrap.sh` and `etcd.sh`, to allow etcd-bootstrap to be used with coreos etcd v3+ images.

# v1.1.1

* Fix vmware docker image.

# v1.1.0

* Add support for discovering cluster members using the GCP API.
* Renames aws image to cloud.
* Updates etcd version to 2.3.8.

# v1.0.0

Add support for discovering cluster members using the VMWare vSphere API.
 Changes: https://github.com/sky-uk/etcd-bootstrap/pull/11

This is a breaking change to the command-line arguments required by the application:

* There is a new mandatory command-line argument, "cloud", which takes values "aws" or "vmware" to select the
  appropriate cloud provider to use as a backend.
* Command-line argument "route53-domain-name" has been renamed to "domain-name" so as to be applicable to multiple cloud
  providers.

# v0.1.1

First official release.
