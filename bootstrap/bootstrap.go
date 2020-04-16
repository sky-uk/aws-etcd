package bootstrap

import (
	"fmt"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/etcd-bootstrap/cloud"
	"github.com/sky-uk/etcd-bootstrap/etcd"
)

// Bootstrapper bootstraps an etcd process by generating a set of Etcd flags for discovery.
type Bootstrapper struct {
	cloud CloudProvider
	cluster  etcdCluster
}

type clusterState string

const (
	// If the node already exists in the cluster state (i.e. other nodes know about it) then the cluster state should
	// retain it's original new status
	newCluster clusterState = "new"
	// If the node is joining a cluster that doesn't know about the node then the cluster state should be existing
	existingCluster clusterState = "existing"
)

// CloudProvider returns instance information for the etcd cluster.
type CloudProvider interface {
	// GetInstances returns all the non-terminated instances that will be part of the etcd cluster.
	GetInstances() []cloud.Instance
	// GetLocalInstance returns the local machine instance.
	GetLocalInstance() cloud.Instance
}

type etcdCluster interface {
	Members() ([]etcd.Member, error)
	AddMember(string) error
	RemoveMember(string) error
}

// New creates a new bootstrapper.
func New(cloud CloudProvider) (*Bootstrapper, error) {
	cluster, err := etcd.New(cloud)
	return &Bootstrapper{
		cloud: cloud,
		cluster:  cluster,
	}, err
}

// GenerateEtcdFlags returns a string containing the generated etcd flags.
func (b *Bootstrapper) GenerateEtcdFlags() (string, error) {
	log.Infof("Generating etcd cluster flags")

	clusterExists, err := b.clusterExists()
	if err != nil {
		return "", err
	}
	if !clusterExists {
		log.Info("No cluster found - treating as an initial node in the new cluster")
		return b.bootstrapNewCluster(), nil
	}

	nodeExistsInCluster, err := b.nodeExistsInCluster()
	if err != nil {
		return "", err
	}
	if nodeExistsInCluster {
		// We treat it as a new cluster in case the cluster hasn't fully bootstrapped yet.
		// etcd will ignore the INITIAL_* flags otherwise, so this should be safe.
		log.Info("Node peer URL already exists - treating as an existing node in a new cluster")
		return b.bootstrapNewCluster(), nil
	}

	log.Info("Node does not exist yet in cluster - joining as a new node")
	return b.bootstrapNewNode()
}

// GenerateEtcdFlagsFile writes etcd flag data to a file.
// It's intended to be sourced in startup scripts.
func (b *Bootstrapper) GenerateEtcdFlagsFile(outputFilename string) error {
	log.Infof("Writing environment variables to %s", outputFilename)
	etcdFLags, err := b.GenerateEtcdFlags()
	if err != nil {
		return err
	}
	return writeToFile(outputFilename, etcdFLags)
}

func (b *Bootstrapper) clusterExists() (bool, error) {
	m, err := b.cluster.Members()
	if err != nil {
		return false, err
	}
	return len(m) > 0, nil
}

func (b *Bootstrapper) nodeExistsInCluster() (bool, error) {
	members, err := b.cluster.Members()
	if err != nil {
		return false, err
	}
	localInstanceURL := peerURL(b.cloud.GetLocalInstance().PrivateIP)

	for _, member := range members {
		if member.PeerURL == localInstanceURL && len(member.Name) > 0 {
			return true, nil
		}
	}

	return false, nil
}

func (b *Bootstrapper) bootstrapNewCluster() string {
	instanceURLs := b.getInstancePeerURLs()
	return b.createEtcdConfig(newCluster, instanceURLs)
}

func (b *Bootstrapper) getInstancePeerURLs() []string {
	instances := b.cloud.GetInstances()

	var peerURLs []string
	for _, i := range instances {
		peerURLs = append(peerURLs, peerURL(i.PrivateIP))
	}

	return peerURLs
}

func (b *Bootstrapper) bootstrapNewNode() (string, error) {
	err := b.reconcileMembers()
	if err != nil {
		return "", err
	}
	clusterURLs, err := b.etcdMemberPeerURLs()
	if err != nil {
		return "", err
	}
	return b.createEtcdConfig(existingCluster, clusterURLs), nil
}

func (b *Bootstrapper) reconcileMembers() error {
	if err := b.removeOldEtcdMembers(); err != nil {
		return err
	}
	return b.addLocalInstanceToEtcd()
}

func (b *Bootstrapper) removeOldEtcdMembers() error {
	memberURLs, err := b.etcdMemberPeerURLs()
	if err != nil {
		return err
	}
	instanceURLs := b.getInstancePeerURLs()

	for _, memberURL := range memberURLs {
		if !contains(instanceURLs, memberURL) {
			log.Infof("Removing %s from etcd member list, not found in cloud provider", memberURL)
			if err := b.cluster.RemoveMember(memberURL); err != nil {
				log.Warnf("Unable to remove old member. This may be due to temporary lack of quorum,"+
					" will ignore: %v", err)
			}
		}
	}

	return nil
}

func (b *Bootstrapper) addLocalInstanceToEtcd() error {
	memberURLs, err := b.etcdMemberPeerURLs()
	if err != nil {
		return err
	}
	localInstanceURL := peerURL(b.cloud.GetLocalInstance().PrivateIP)

	if !contains(memberURLs, localInstanceURL) {
		log.Infof("Adding local instance %s to the etcd member list", localInstanceURL)
		if err := b.cluster.AddMember(localInstanceURL); err != nil {
			return fmt.Errorf("unexpected error when adding new member URL %s: %v", localInstanceURL, err)
		}
	}

	return nil
}

func (b *Bootstrapper) etcdMemberPeerURLs() ([]string, error) {
	members, err := b.cluster.Members()
	if err != nil {
		return nil, err
	}
	var memberURLs []string
	for _, member := range members {
		memberURLs = append(memberURLs, member.PeerURL)
	}
	return memberURLs, nil
}

func (b *Bootstrapper) createEtcdConfig(state clusterState, availablePeerURLs []string) string {
	var envs []string

	envs = append(envs, "ETCD_INITIAL_CLUSTER_STATE="+string(state))
	initialCluster := b.constructInitialCluster(availablePeerURLs)
	envs = append(envs, fmt.Sprintf("ETCD_INITIAL_CLUSTER=%s", initialCluster))

	local := b.cloud.GetLocalInstance()
	envs = append(envs, fmt.Sprintf("ETCD_NAME=%s", local.InstanceID))
	envs = append(envs, fmt.Sprintf("ETCD_INITIAL_ADVERTISE_PEER_URLS=%s", peerURL(local.PrivateIP)))
	envs = append(envs, fmt.Sprintf("ETCD_LISTEN_PEER_URLS=%s", peerURL(local.PrivateIP)))
	envs = append(envs, fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=%s,%s", clientURL(local.PrivateIP), clientURL("127.0.0.1")))
	envs = append(envs, fmt.Sprintf("ETCD_ADVERTISE_CLIENT_URLS=%s", clientURL(local.PrivateIP)))

	return strings.Join(envs, "\n") + "\n"
}

func (b *Bootstrapper) constructInitialCluster(availablePeerURLs []string) string {
	var initialCluster []string
	for _, instance := range b.cloud.GetInstances() {
		instancePeerURL := peerURL(instance.PrivateIP)
		if contains(availablePeerURLs, instancePeerURL) {
			initialCluster = append(initialCluster,
				fmt.Sprintf("%s=%s", instance.InstanceID, instancePeerURL))
		}
	}

	return strings.Join(initialCluster, ",")
}

func writeToFile(outputFilename string, data string) error {
	return ioutil.WriteFile(outputFilename, []byte(data), 0644)
}

func peerURL(ip string) string {
	return fmt.Sprintf("http://%s:2380", ip)
}

func clientURL(ip string) string {
	return fmt.Sprintf("http://%s:2379", ip)
}

func contains(strings []string, value string) bool {
	for _, s := range strings {
		if value == s {
			return true
		}
	}
	return false
}
