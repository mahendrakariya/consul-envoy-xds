package eds_test

import (
	"github.com/gojektech/consul-envoy-xds/eds"
	"testing"

	cp "github.com/envoyproxy/go-control-plane/api"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockConsulAgent struct {
	mock.Mock
}

func (m MockConsulAgent) Locality() *cp.Locality {
	args := m.Called()
	return args.Get(0).(*cp.Locality)
}

func (m MockConsulAgent) CatalogServiceEndpoints(serviceName string) ([]*api.CatalogService, error) {
	args := m.Called(serviceName)
	return args.Get(0).([]*api.CatalogService), args.Error(1)
}

func (m MockConsulAgent) WatchParams() map[string]string {
	args := m.Called()
	return args.Get(0).(map[string]string)
}

func TestShouldHaveCLAUsingAgentCatalogServiceEndpoints(t *testing.T) {
	agent := &MockConsulAgent{}
	agent.On("Locality").Return(&cp.Locality{Region: "foo-region"})
	agent.On("CatalogServiceEndpoints", "foo-service").Return([]*api.CatalogService{{ServiceAddress: "foo1", ServicePort: 1234}, {ServiceAddress: "foo2", ServicePort: 1234}}, nil)
	endpoint := eds.NewEndpoint("foo-service", agent)
	cla := endpoint.CLA()

	assert.Equal(t, "foo-service", cla.ClusterName)
	assert.Equal(t, float64(0), cla.Policy.DropOverload)
	localityEndpoint := cla.Endpoints[0]
	assert.Equal(t, "foo-region", localityEndpoint.Locality.Region)
	assert.Equal(t, "socket_address:<address:\"foo1\" port_value:1234 > ", localityEndpoint.LbEndpoints[0].Endpoint.Address.String())
	assert.Equal(t, "socket_address:<address:\"foo2\" port_value:1234 > ", localityEndpoint.LbEndpoints[1].Endpoint.Address.String())
}

func TestShouldSetAgentBasedWatcherParamsInEndpointWatchPlan(t *testing.T) {
	agent := &MockConsulAgent{}
	endpoint := eds.NewEndpoint("foo-service", agent)
	agent.On("WatchParams").Return(map[string]string{"datacenter": "dc-foo-01", "token": "token-foo-01"})

	plan, _ := endpoint.WatchPlan(func(*cp.ClusterLoadAssignment) {
	})

	assert.Equal(t, "dc-foo-01", plan.Datacenter)
	assert.Equal(t, "token-foo-01", plan.Token)
}

func TestShouldSetEndpointWatchPlanHandler(t *testing.T) {
	agent := &MockConsulAgent{}
	agent.On("Locality").Return(&cp.Locality{Region: "foo-region"})
	agent.On("CatalogServiceEndpoints", "foo-service").Return([]*api.CatalogService{{ServiceAddress: "foo1", ServicePort: 1234}, {ServiceAddress: "foo2", ServicePort: 1234}}, nil)

	endpoint := eds.NewEndpoint("foo-service", agent)
	agent.On("WatchParams").Return(map[string]string{"datacenter": "dc-foo-01", "token": "token-foo-01"})
	var handlerCalled bool
	var capture *cp.ClusterLoadAssignment
	plan, _ := endpoint.WatchPlan(func(cla *cp.ClusterLoadAssignment) {
		handlerCalled = true
		capture = cla
	})

	plan.Handler(112345, nil)
	assert.True(t, handlerCalled)
	assert.Equal(t, "foo-service", capture.ClusterName)
}
