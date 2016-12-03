package bootstrap

import (
	"testing"

	"strings"

	"errors"

	"github.com/sky-uk/etcd-bootstrap/lib/asg"
	"github.com/sky-uk/etcd-bootstrap/lib/etcdcluster"
	"github.com/stretchr/testify/assert"
)

type testASG struct {
	instances []asg.Instance
	local     asg.Instance
}

func (a *testASG) GetInstances() []asg.Instance {
	return a.instances
}

func (a *testASG) GetLocalInstance() asg.Instance {
	return a.local
}

type testCluster struct {
	members       []etcdcluster.Member
	removedMember []string
	addedMember   []string
}

func (e *testCluster) Members() []etcdcluster.Member {
	return e.members
}

func (e *testCluster) RemoveMember(peerURL string) error {
	var idx int
	for i, m := range e.members {
		if m.PeerURL == peerURL {
			idx = i
		}
	}
	e.members = append(e.members[:idx], e.members[idx+1:]...)

	e.removedMember = append(e.removedMember, peerURL)
	return errors.New("Test that remove member error is ignored")
}

func (e *testCluster) AddMember(peerURL string) error {
	e.members = append(e.members, etcdcluster.Member{PeerURL: peerURL})

	e.addedMember = append(e.addedMember, peerURL)
	return nil
}

func TestCreateNewCluster(t *testing.T) {
	assert := assert.New(t)

	testASG := &testASG{}
	testASG.instances = []asg.Instance{
		{InstanceID: "e1", PrivateIP: "10.50.99.1"},
		{InstanceID: "e2", PrivateIP: "10.50.199.1"},
		{InstanceID: "e3", PrivateIP: "10.50.155.1"}}
	testASG.local = asg.Instance{InstanceID: "e2", PrivateIP: "10.50.199.1"}

	etcdCluster := &testCluster{}

	bootstrapper := New(testASG, etcdCluster)
	vars := strings.Split(bootstrapper.Bootstrap(), "\n")

	assert.Contains(vars, "ETCD_INITIAL_CLUSTER_STATE=new")
	assert.Contains(vars, "ETCD_INITIAL_CLUSTER=e1=http://10.50.99.1:2380,"+
		"e2=http://10.50.199.1:2380,e3=http://10.50.155.1:2380")
	assert.Contains(vars, "ETCD_INITIAL_ADVERTISE_PEER_URLS=http://10.50.199.1:2380")
	assert.Contains(vars, "ETCD_NAME=e2")
	assert.Contains(vars, "ETCD_LISTEN_PEER_URLS=http://10.50.199.1:2380")
	assert.Contains(vars, "ETCD_LISTEN_CLIENT_URLS=http://10.50.199.1:2379,http://127.0.0.1:2379")
	assert.Contains(vars, "ETCD_ADVERTISE_CLIENT_URLS=http://10.50.199.1:2379")
}

func TestExistingCluster(t *testing.T) {
	assert := assert.New(t)

	testASG := &testASG{}
	testASG.instances = []asg.Instance{
		{InstanceID: "e1", PrivateIP: "10.50.99.1"},
		{InstanceID: "e2", PrivateIP: "10.50.199.1"},
		{InstanceID: "e3", PrivateIP: "10.50.155.1"}}
	testASG.local = asg.Instance{InstanceID: "e2", PrivateIP: "10.50.199.1"}

	etcdCluster := &testCluster{}
	etcdCluster.members = []etcdcluster.Member{
		{
			Name:    "e1",
			PeerURL: "http://10.50.99.1:2380",
		},
		{
			Name:    "e2",
			PeerURL: "http://10.50.199.1:2380",
		},
		{
			Name:    "e3",
			PeerURL: "http://10.50.155.1:2380",
		},
	}

	bootstrapper := New(testASG, etcdCluster)
	vars := strings.Split(bootstrapper.Bootstrap(), "\n")

	assert.Contains(vars, "ETCD_INITIAL_CLUSTER_STATE=new")
	assert.Contains(vars, "ETCD_INITIAL_CLUSTER=e1=http://10.50.99.1:2380,"+
		"e2=http://10.50.199.1:2380,e3=http://10.50.155.1:2380")
	assert.Contains(vars, "ETCD_INITIAL_ADVERTISE_PEER_URLS=http://10.50.199.1:2380")
	assert.Contains(vars, "ETCD_NAME=e2")
	assert.Contains(vars, "ETCD_LISTEN_PEER_URLS=http://10.50.199.1:2380")
	assert.Contains(vars, "ETCD_LISTEN_CLIENT_URLS=http://10.50.199.1:2379,http://127.0.0.1:2379")
	assert.Contains(vars, "ETCD_ADVERTISE_CLIENT_URLS=http://10.50.199.1:2379")
}

func TestJoinAnExistingCluster(t *testing.T) {
	assert := assert.New(t)

	testASG := &testASG{}
	testASG.instances = []asg.Instance{
		{InstanceID: "e1", PrivateIP: "10.50.99.1"},
		{InstanceID: "e2", PrivateIP: "10.50.199.1"},
		{InstanceID: "e3", PrivateIP: "10.50.155.1"}}
	testASG.local = asg.Instance{InstanceID: "e2", PrivateIP: "10.50.199.1"}

	etcdCluster := &testCluster{}
	etcdCluster.members = []etcdcluster.Member{
		{
			Name:    "e1",
			PeerURL: "http://10.50.99.1:2380",
		},
		{
			Name:    "ea",
			PeerURL: "http://10.50.65.2:2380",
		},
		{
			Name:    "eb",
			PeerURL: "http://10.50.44.44:2380",
		},
	}

	bootstrapper := New(testASG, etcdCluster)
	vars := strings.Split(bootstrapper.Bootstrap(), "\n")

	assert.Contains(etcdCluster.addedMember, "http://10.50.199.1:2380")
	assert.NotContains(etcdCluster.addedMember, "http://10.50.155.1:2380",
		"Should only add itself, to prevent stuck quorum errors")
	assert.Contains(etcdCluster.removedMember, "http://10.50.65.2:2380")
	assert.Contains(etcdCluster.removedMember, "http://10.50.44.44:2380")
	assert.Len(etcdCluster.addedMember, 1)
	assert.Len(etcdCluster.removedMember, 2)

	assert.Contains(vars, "ETCD_INITIAL_CLUSTER_STATE=existing")
	assert.Contains(vars, "ETCD_INITIAL_CLUSTER=e1=http://10.50.99.1:2380,e2=http://10.50.199.1:2380",
		"Initial cluster should only include the new local node and existing node, otherwise we'll get a "+
			"'member count unequal' error at etcd startup.")
}

func TestJoinAnExistingClusterWhenPartiallyInitialised(t *testing.T) {
	assert := assert.New(t)

	testASG := &testASG{}
	testASG.instances = []asg.Instance{
		{InstanceID: "e1", PrivateIP: "10.50.99.1"},
		{InstanceID: "e2", PrivateIP: "10.50.199.1"},
		{InstanceID: "e3", PrivateIP: "10.50.155.1"}}
	testASG.local = asg.Instance{InstanceID: "e3", PrivateIP: "10.50.155.1"}

	etcdCluster := &testCluster{}
	etcdCluster.members = []etcdcluster.Member{
		{
			Name:    "e1",
			PeerURL: "http://10.50.99.1:2380",
		},
		{
			Name:    "e2",
			PeerURL: "http://10.50.199.1:2380",
		},
		{
			Name:    "",
			PeerURL: "http://10.50.155.1:2380",
		},
	}

	bootstrapper := New(testASG, etcdCluster)
	vars := strings.Split(bootstrapper.Bootstrap(), "\n")

	assert.Contains(vars, "ETCD_INITIAL_CLUSTER_STATE=existing",
		"Should join existing cluster as it hasn't initialised yet (name is blank), despite having its peerURL already added.")
	assert.Contains(vars, "ETCD_INITIAL_CLUSTER=e1=http://10.50.99.1:2380,e2=http://10.50.199.1:2380,e3=http://10.50.155.1:2380")
}
