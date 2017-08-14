package main

import (
	"flag"

	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/sky-uk/etcd-bootstrap/lib"
	"github.com/sky-uk/etcd-bootstrap/lib/members"
)

var (
	cloud string
    vmwareUsername string
    vmwarePassword string
    vmwareHost string
    vmwarePort string
    vmwareInsecure bool
    vmwareAttempts uint
    vmwareVMName string
    vmwareEnv string
    vmwareRole string
    outputFilename string
    zoneID string
    domainName string
)

const (
	defaultVmwarePort     = "443"
	defaultVmwareInsecure = true
	defaultVmwareAttempts = 3
	defaultOutputFilename = "/var/run/etcd-bootstrap.conf"
)

func init() {
	flag.StringVar(&cloud, "cloud", "",
	    "cloud provider to use.  Required, and must be one of 'aws' or 'vmware'")
	flag.StringVar(&vmwareUsername, "vmware-username", "",
		"username for vSphere API")
	flag.StringVar(&vmwarePassword, "vmware-password", "",
		"plaintext password for vSphere API")
	flag.StringVar(&vmwareHost, "vmware-host", "",
		"host address for vSphere API")
	flag.StringVar(&vmwarePort, "vmware-port", defaultVmwarePort,
		"port for vSphere API.  Defaults to 443")
	flag.BoolVar(&vmwareInsecure, "vmware-insecure", defaultVmwareInsecure,
		"flag to indicate if vSphere API uses a self-signed certificate")
	flag.UintVar(&vmwareAttempts, "vmware-attempts", defaultVmwareAttempts,
		"number of attempts to make against the vSphere SOAP API (in case of temporary failure)")
	flag.StringVar(&vmwareVMName, "vmware-vm-name", "",
		"node name in vSphere of this VM")
	flag.StringVar(&vmwareEnv, "vmware-environment", "",
		"value of the 'tags_environment' extra configuration option in vSphere to filter nodes by")
	flag.StringVar(&vmwareRole, "vmware-role", "",
		"value of the 'tags_role' extra configuration option in vSphere to filter nodes by")
	flag.StringVar(&outputFilename, "o", defaultOutputFilename,
		"location to write environment variables for etcd to use")
	flag.StringVar(&zoneID, "route53-zone-id", "",
		"route53 zone ID to update with the IP addresses of the etcd auto scaling group")
	flag.StringVar(&domainName, "route53-domain-name", "",
		"domain name to update inside the Route53 zone")
}

func main() {
	flag.Parse()

	validateArguments()

	var bootstrapper bootstrap.Bootstrapper
	var err error
	if cloud == "vmware" {
		config := &members.VmwareConfig{
			User:              vmwareUsername,
			Password:          vmwarePassword,
			VCenterHost:       vmwareHost,
			VCenterPort:       vmwarePort,
			InsecureFlag:      vmwareInsecure,
			RoundTripperCount: vmwareAttempts,
			VMName:            vmwareVMName,
			Environment:	   vmwareEnv,
			Role:              vmwareRole,
		}

		bootstrapper, err = bootstrap.LocalVMWare(config)
	} else {
		bootstrapper, err = bootstrap.LocalASG()
	}
	if err != nil {
		log.Fatalf("Unable to initialise bootstrapper: %v", err)
	}

	etcdOut, err := bootstrapper.BootstrapEtcdFlags()
	if err != nil {
		log.Fatalf("Unable to bootstrap etcd flags: %v", err)
	}

	out := "# created by etcd-bootstrap\n"
	out += etcdOut

	log.Infof("Writing environment variables to %s", outputFilename)
	if err := ioutil.WriteFile(outputFilename, []byte(out), 0644); err != nil {
		log.Fatalf("Unable to write to %s: %v", outputFilename, err)
	}

	if zoneID != "" && domainName != "" {
		log.Infof("Adding etcd IPs to %q in route53 zone %q", domainName, zoneID)
		if err := bootstrapper.BootstrapRoute53(zoneID, domainName); err != nil {
			log.Fatalf("Unable to bootstrap route53: %v", err)
		}
	}
}

func validateArguments() {
	if cloud == "" || (cloud != "aws" && cloud != "vmware") {
		log.Fatal("Cloud argument must be one of 'aws' or 'vmware'")
	}
}
