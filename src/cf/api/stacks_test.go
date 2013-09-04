package api_test

import (
	. "cf/api"
	"cf/configuration"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testhelpers"
	"testing"
)

var singleStackResponse = testhelpers.TestResponse{Status: http.StatusOK, Body: `
{
"resources": [
    {
      "metadata": {
        "guid": "custom-linux-guid"
      },
      "entity": {
        "name": "custom-linux"
      }
    }
  ]
}`}

var singleStackEndpoint = testhelpers.CreateEndpoint(
	"GET",
	"/v2/stacks?q=name%3Alinux",
	nil,
	singleStackResponse,
)

func TestStacksFindByName(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(singleStackEndpoint))
	defer ts.Close()

	repo := CloudControllerStackRepository{}
	config := &configuration.Configuration{
		AccessToken: "BEARER my_access_token",
		Target:      ts.URL,
	}

	stack, err := repo.FindByName(config, "linux")
	assert.NoError(t, err)
	assert.Equal(t, stack.Name, "custom-linux")
	assert.Equal(t, stack.Guid, "custom-linux-guid")

	stack, err = repo.FindByName(config, "stack that does not exist")
	assert.Error(t, err)
}