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