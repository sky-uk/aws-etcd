package bootstrap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sky-uk/etcd-bootstrap/cloud"
	"github.com/sky-uk/etcd-bootstrap/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	localPrivateIP  = "192.168.0.100"
	localInstanceID = "test-local-instance"
)

var (
	localPeerURL   = fmt.Sprintf("http://%v:2380", localPrivateIP)
	localClientURL = fmt.Sprintf("http://%v:2379", localPrivateIP)
)

// TestBootstrap to register the test suite
func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap")
}

var _ = Describe("Bootstrap", func() {
	var (
		cloudProvider CloudProvider
		etcdCluster   EtcdCluster
	)

	BeforeEach(func() {
		By("Returning the constant instance values")
		cloudProvider = CloudProvider{
			MockGetLocalInstance: GetLocalInstance{
				GetLocalInstance: cloud.Instance{
					Name:     localInstanceID,
					Endpoint: localPrivateIP,
				},
			},
		}

		By("Only calling AddMember() from the local instance using the constant values")
		etcdCluster = EtcdCluster{
			MockAddMember: AddMember{
				ExpectedInput: localPeerURL,
			},
		}
	})

	It("helper functions work", func() {
		Expect(peerURL(localPrivateIP)).To(Equal(localPeerURL))
		Expect(clientURL(localPrivateIP)).To(Equal(localClientURL))
	})

	It("fails when it cannot get etcd members", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
		}

		By("Returning an error when getting the list of etcd members")
		etcdCluster.MockMembers.Err = fmt.Errorf("failed to get etcd members")
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		_, err := bootstrapperClient.GenerateEtcdFlags()
		Expect(err).ToNot(BeNil())
	})

	It("does not fail when it cannot remove member", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
		}

		By("Returning an etcd member list requiring an update")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{
			{
				Name:    "test-remove-instance-id-1",
				PeerURL: "http://192.168.0.1:2380",
			},
		}

		By("Expecting to receive a call to remove a test instance")
		etcdCluster.MockRemoveMember.ExpectedInputs = []string{"http://192.168.0.1:2380"}

		By("Returning an error when trying to remove an etcd member")
		etcdCluster.MockRemoveMember.Err = fmt.Errorf("failed to remove etcd members")
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		_, err := bootstrapperClient.GenerateEtcdFlags()

		By("Do not fail as it may be down to an etcd quorum issue")
		Expect(err).To(BeNil())
	})

	It("fails when it cannot add etcd member", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
			{
				Name:     "test-add-instance-id-1",
				Endpoint: "192.168.0.1",
			},
		}

		By("Returning an etcd member list requiring an update")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{
			{
				Name:    "test-add-instance-id-1",
				PeerURL: "http://192.168.0.1:2380",
			},
		}

		By("Returning an error when attempting to list all etcd members")
		etcdCluster.MockAddMember.Err = fmt.Errorf("failed to add etcd member")
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		_, err := bootstrapperClient.GenerateEtcdFlags()

		By("Do not fail as it may be down to an etcd quorum issue")
		Expect(err).ToNot(BeNil())
	})

	It("new cluster", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
			{
				Name:     "test-new-cluster-instance-id-1",
				Endpoint: "192.168.0.1",
			},
			{
				Name:     "test-new-cluster-instance-id-2",
				Endpoint: "192.168.0.2",
			},
		}

		By("Returning a list of etcd members that is empty")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{}
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		etcdFlags, err := bootstrapperClient.GenerateEtcdFlags()
		flags := strings.Split(etcdFlags, "\n")
		Expect(err).To(BeNil())
		Expect(flags).To(ContainElement("ETCD_INITIAL_CLUSTER_STATE=new"))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_INITIAL_CLUSTER=%v=%v,"+
			"test-new-cluster-instance-id-1=http://192.168.0.1:2380,"+
			"test-new-cluster-instance-id-2=http://192.168.0.2:2380", localInstanceID, localPeerURL)))
		Expect(flags).To(ContainElement("ETCD_NAME=" + localInstanceID))
		Expect(flags).To(ContainElement("ETCD_INITIAL_ADVERTISE_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement("ETCD_LISTEN_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=%v,%v", localClientURL, clientURL("127.0.0.1"))))
		Expect(flags).To(ContainElement("ETCD_ADVERTISE_CLIENT_URLS=" + localClientURL))
	})

	It("an existing cluster", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
			{
				Name:     "test-existing-cluster-instance-id-1",
				Endpoint: "192.168.0.1",
			},
			{
				Name:     "test-existing-cluster-instance-id-2",
				Endpoint: "192.168.0.2",
			},
		}

		By("Returning a list of etcd members that contains too many members but does not include the local instance")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{
			{
				Name:    localInstanceID,
				PeerURL: localPeerURL,
			},
			{
				Name:    "test-existing-cluster-instance-id-1",
				PeerURL: "http://192.168.0.1:2380",
			},
			{
				Name:    "test-existing-cluster-instance-id-2",
				PeerURL: "http://192.168.0.2:2380",
			},
		}
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		etcdFlags, err := bootstrapperClient.GenerateEtcdFlags()
		flags := strings.Split(etcdFlags, "\n")
		Expect(err).To(BeNil())
		Expect(flags).To(ContainElement("ETCD_INITIAL_CLUSTER_STATE=new"))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_INITIAL_CLUSTER=%v=%v,"+
			"test-existing-cluster-instance-id-1=http://192.168.0.1:2380,"+
			"test-existing-cluster-instance-id-2=http://192.168.0.2:2380", localInstanceID, localPeerURL)))
		Expect(flags).To(ContainElement("ETCD_NAME=" + localInstanceID))
		Expect(flags).To(ContainElement("ETCD_INITIAL_ADVERTISE_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement("ETCD_LISTEN_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=%v,%v", localClientURL, clientURL("127.0.0.1"))))
		Expect(flags).To(ContainElement("ETCD_ADVERTISE_CLIENT_URLS=" + localClientURL))
	})

	It("an existing cluster where a node needs replacing", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
			{
				Name:     "test-existing-cluster-instance-id-2",
				Endpoint: "192.168.0.2",
			},
			{
				Name:     "test-existing-cluster-instance-id-3",
				Endpoint: "192.168.0.3",
			},
		}

		By("Returning a list of etcd members that contains too many members but does not include the local instance")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{
			{
				Name:    "test-existing-cluster-old-instance-id-1",
				PeerURL: "http://192.168.0.1:2380",
			},
			{
				Name:    "test-existing-cluster-instance-id-2",
				PeerURL: "http://192.168.0.2:2380",
			},
			{
				Name:    "test-existing-cluster-instance-id-3",
				PeerURL: "http://192.168.0.3:2380",
			},
		}

		By("Expecting a RemoveMember() call to be made with the old instance PeerURL")
		etcdCluster.MockRemoveMember.ExpectedInputs = []string{"http://192.168.0.1:2380"}
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		etcdFlags, err := bootstrapperClient.GenerateEtcdFlags()
		flags := strings.Split(etcdFlags, "\n")
		Expect(err).To(BeNil())
		Expect(flags).To(ContainElement("ETCD_INITIAL_CLUSTER_STATE=existing"))
		Expect(flags).To(ContainElement("ETCD_INITIAL_CLUSTER=test-existing-cluster-instance-id-2=http://192.168.0.2:2380," +
			"test-existing-cluster-instance-id-3=http://192.168.0.3:2380"))
		Expect(flags).To(ContainElement("ETCD_NAME=" + localInstanceID))
		Expect(flags).To(ContainElement("ETCD_INITIAL_ADVERTISE_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement("ETCD_LISTEN_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=%v,%v", localClientURL, clientURL("127.0.0.1"))))
		Expect(flags).To(ContainElement("ETCD_ADVERTISE_CLIENT_URLS=" + localClientURL))
	})

	It("an existing cluster when partially initialised", func() {
		By("Returning some instances including the local instance")
		cloudProvider.MockGetInstances.GetInstancesOutput = []cloud.Instance{
			{
				Name:     localInstanceID,
				Endpoint: localPrivateIP,
			},
			{
				Name:     "test-existing-cluster-partially-initialised-instance-id-1",
				Endpoint: "192.168.0.1",
			},
			{
				Name:     "test-existing-cluster-partially-initialised-instance-id-2",
				Endpoint: "192.168.0.2",
			},
		}

		By("Returning a list of etcd members that contains too many members but does not include the local instance")
		etcdCluster.MockMembers.MembersOutput = []etcd.Member{
			{
				Name:    "",
				PeerURL: localPeerURL,
			},
			{
				Name:    "test-existing-cluster-partially-initialised-instance-id-1",
				PeerURL: "http://192.168.0.1:2380",
			},
			{
				Name:    "test-existing-cluster-partially-initialised-instance-id-2",
				PeerURL: "http://192.168.0.2:2380",
			},
		}
		bootstrapperClient := Bootstrapper{
			instances:     cloudProvider,
			localInstance: cloudProvider,
			cluster:       etcdCluster,
		}

		etcdFlags, err := bootstrapperClient.GenerateEtcdFlags()
		flags := strings.Split(etcdFlags, "\n")
		Expect(err).To(BeNil())

		By("Joining the existing cluster as the node has not initialised fully yet")
		Expect(flags).To(ContainElement("ETCD_INITIAL_CLUSTER_STATE=existing"))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_INITIAL_CLUSTER=%v=%v,"+
			"test-existing-cluster-partially-initialised-instance-id-1=http://192.168.0.1:2380,"+
			"test-existing-cluster-partially-initialised-instance-id-2=http://192.168.0.2:2380", localInstanceID, localPeerURL)))
		Expect(flags).To(ContainElement("ETCD_NAME=" + localInstanceID))
		Expect(flags).To(ContainElement("ETCD_INITIAL_ADVERTISE_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement("ETCD_LISTEN_PEER_URLS=" + localPeerURL))
		Expect(flags).To(ContainElement(fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=%v,%v", localClientURL, clientURL("127.0.0.1"))))
		Expect(flags).To(ContainElement("ETCD_ADVERTISE_CLIENT_URLS=" + localClientURL))
	})
})

// EtcdCluster for mocking calls to the etcd cluster package client
type EtcdCluster struct {
	MockMembers      Members
	MockRemoveMember RemoveMember
	MockAddMember    AddMember
}

// Members sets the expected output for Members() on EtcdCluster
type Members struct {
	MembersOutput []etcd.Member
	Err           error
}

// Members mocks the etcd cluster package client
func (t EtcdCluster) Members() ([]etcd.Member, error) {
	return t.MockMembers.MembersOutput, t.MockMembers.Err
}

// RemoveMember sets the expected input for RemoveMember() on EtcdCluster
type RemoveMember struct {
	ExpectedInputs []string
	Err            error
}

// RemoveMember mocks the etcd cluster package client
func (t EtcdCluster) RemoveMember(peerURL string) error {
	Expect(t.MockRemoveMember.ExpectedInputs).To(ContainElement(peerURL))
	return t.MockRemoveMember.Err
}

// AddMember sets the expected input for AddMember() on EtcdCluster
type AddMember struct {
	ExpectedInput string
	Err           error
}

// AddMember mocks the etcd cluster package client
func (t EtcdCluster) AddMember(peerURL string) error {
	Expect(peerURL).To(Equal(t.MockAddMember.ExpectedInput))
	return t.MockAddMember.Err
}

// CloudProvider for mocking calls to an etcd-bootstrap cloud provider
type CloudProvider struct {
	MockGetInstances     GetInstances
	MockGetLocalInstance GetLocalInstance
}

// GetInstances sets the expected output for GetInstances() on CloudProvider
type GetInstances struct {
	GetInstancesOutput []cloud.Instance
	Error              error
}

// GetInstances mocks the etcd-bootstrap cloud provider
func (t CloudProvider) GetInstances() ([]cloud.Instance, error) {
	return t.MockGetInstances.GetInstancesOutput, t.MockGetInstances.Error
}

// GetLocalInstance sets the expected output for GetLocalInstance() on CloudProvider
type GetLocalInstance struct {
	GetLocalInstance cloud.Instance
	Error            error
}

// GetLocalInstance mocks the etcd-bootstrap cloud provider
func (t CloudProvider) GetLocalInstance() (cloud.Instance, error) {
	return t.MockGetLocalInstance.GetLocalInstance, t.MockGetLocalInstance.Error
}
