package api_test

import (
	. "cf/api"
	"cf/models"
	"cf/net"
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	testapi "testhelpers/api"
	testconfig "testhelpers/configuration"
	testnet "testhelpers/net"
)

var _ = Describe("UserRepository", func() {
	Describe("listing the users with a given role", func() {
		It("lists the users in an organization with a given role", func() {
			ccReqs, uaaReqs := createUsersByRoleEndpoints("/v2/organizations/my-org-guid/managers")

			cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo(ccReqs, uaaReqs)
			defer cc.Close()
			defer uaa.Close()

			users, apiResponse := repo.ListUsersInOrgForRole("my-org-guid", models.ORG_MANAGER)

			Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
			Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
			Expect(apiResponse.IsSuccessful()).To(BeTrue())

			Expect(len(users)).To(Equal(3))
			Expect(users[0].Guid).To(Equal("user-1-guid"))
			Expect(users[0].Username).To(Equal("Super user 1"))
			Expect(users[1].Guid).To(Equal("user-2-guid"))
			Expect(users[1].Username).To(Equal("Super user 2"))
		})

		It("lists the users in a space with a given role", func() {
			ccReqs, uaaReqs := createUsersByRoleEndpoints("/v2/spaces/my-space-guid/managers")

			cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo(ccReqs, uaaReqs)
			defer cc.Close()
			defer uaa.Close()

			users, apiResponse := repo.ListUsersInSpaceForRole("my-space-guid", models.SPACE_MANAGER)

			Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
			Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
			Expect(apiResponse.IsSuccessful()).To(BeTrue())

			Expect(len(users)).To(Equal(3))
			Expect(users[0].Guid).To(Equal("user-1-guid"))
			Expect(users[0].Username).To(Equal("Super user 1"))
			Expect(users[1].Guid).To(Equal("user-2-guid"))
			Expect(users[1].Username).To(Equal("Super user 2"))
		})

		It("does not make a request to the UAA when the cloud controller returns an error", func() {
			ccReqs := []testnet.TestRequest{
				testapi.NewCloudControllerTestRequest(testnet.TestRequest{
					Method: "GET",
					Path:   "/v2/organizations/my-org-guid/managers",
					Response: testnet.TestResponse{
						Status: http.StatusGatewayTimeout,
					},
				}),
			}

			cc, ccHandler, _, _, repo := createUsersRepo(ccReqs, []testnet.TestRequest{})
			defer cc.Close()

			_, apiResponse := repo.ListUsersInOrgForRole("my-org-guid", models.ORG_MANAGER)

			Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
			Expect(apiResponse.StatusCode).To(Equal(http.StatusGatewayTimeout))
		})

		It("returns an error when the UAA endpoint cannot be determined", func() {
			ccReqs, _ := createUsersByRoleEndpoints("/v2/organizations/my-org-guid/managers")

			ts, _ := testnet.NewTLSServer(ccReqs)
			defer ts.Close()
			configRepo := testconfig.NewRepositoryWithDefaults()
			configRepo.SetApiEndpoint(ts.URL)

			ccGateway := net.NewCloudControllerGateway()
			uaaGateway := net.NewUAAGateway()
			endpointRepo := &testapi.FakeEndpointRepo{}
			endpointRepo.UAAEndpointReturns.ApiResponse = net.NewApiResponseWithError("Failed to get endpoint!", errors.New("Failed!"))

			repo := NewCloudControllerUserRepository(configRepo, uaaGateway, ccGateway, endpointRepo)

			_, apiResponse := repo.ListUsersInOrgForRole("my-org-guid", models.ORG_MANAGER)

			Expect(apiResponse).To(Equal(endpointRepo.UAAEndpointReturns.ApiResponse))
		})
	})

	It("TestFindByUsername", func() {
		usersResponse := `{ "resources": [{ "id": "my-guid", "userName": "my-full-username" }]}`
		uaaReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "GET",
			Path:     "/Users?attributes=id,userName&filter=userName+Eq+%22damien%2Buser1%40pivotallabs.com%22",
			Response: testnet.TestResponse{Status: http.StatusOK, Body: usersResponse},
		})

		uaa, handler, repo := createUsersRepoWithoutCCEndpoints([]testnet.TestRequest{uaaReq})
		defer uaa.Close()

		user, apiResponse := repo.FindByUsername("damien+user1@pivotallabs.com")
		Expect(handler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsSuccessful()).To(BeTrue())

		expectedUserFields := models.UserFields{}
		expectedUserFields.Username = "my-full-username"
		expectedUserFields.Guid = "my-guid"
		Expect(user).To(Equal(expectedUserFields))
	})

	It("TestFindByUsernameWhenNotFound", func() {
		uaaReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "GET",
			Path:     "/Users?attributes=id,userName&filter=userName+Eq+%22my-user%22",
			Response: testnet.TestResponse{Status: http.StatusOK, Body: `{"resources": []}`},
		})

		uaa, handler, repo := createUsersRepoWithoutCCEndpoints([]testnet.TestRequest{uaaReq})
		defer uaa.Close()

		_, apiResponse := repo.FindByUsername("my-user")
		Expect(handler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsError()).To(BeFalse())
		Expect(apiResponse.IsNotFound()).To(BeTrue())
		Expect(apiResponse.Message).To(ContainSubstring("User my-user not found"))
	})

	It("TestCreateUser", func() {
		ccReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "POST",
			Path:     "/v2/users",
			Matcher:  testnet.RequestBodyMatcher(`{"guid":"my-user-guid"}`),
			Response: testnet.TestResponse{Status: http.StatusCreated},
		})

		uaaReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method: "POST",
			Path:   "/Users",
			Matcher: testnet.RequestBodyMatcher(`{
			"userName":"my-user",
			"emails":[{"value":"my-user"}],
			"password":"my-password",
			"name":{
				"givenName":"my-user",
				"familyName":"my-user"}
			}`),
			Response: testnet.TestResponse{
				Status: http.StatusCreated,
				Body:   `{"id":"my-user-guid"}`,
			},
		})

		cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo([]testnet.TestRequest{ccReq}, []testnet.TestRequest{uaaReq})
		defer cc.Close()
		defer uaa.Close()

		apiResponse := repo.Create("my-user", "my-password")
		Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
		Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsNotSuccessful()).To(BeFalse())
	})

	It("TestDeleteUser", func() {
		ccReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "DELETE",
			Path:     "/v2/users/my-user-guid",
			Response: testnet.TestResponse{Status: http.StatusOK},
		})

		uaaReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "DELETE",
			Path:     "/Users/my-user-guid",
			Response: testnet.TestResponse{Status: http.StatusOK},
		})

		cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo([]testnet.TestRequest{ccReq}, []testnet.TestRequest{uaaReq})
		defer cc.Close()
		defer uaa.Close()

		apiResponse := repo.Delete("my-user-guid")
		Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
		Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsSuccessful()).To(BeTrue())
	})

	It("TestDeleteUserWhenNotFoundOnTheCloudController", func() {
		ccReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "DELETE",
			Path:     "/v2/users/my-user-guid",
			Response: testnet.TestResponse{Status: http.StatusNotFound, Body: `{"code": 20003, "description": "The user could not be found"}`},
		})

		uaaReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method:   "DELETE",
			Path:     "/Users/my-user-guid",
			Response: testnet.TestResponse{Status: http.StatusOK},
		})

		cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo([]testnet.TestRequest{ccReq}, []testnet.TestRequest{uaaReq})
		defer cc.Close()
		defer uaa.Close()

		apiResponse := repo.Delete("my-user-guid")
		Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
		Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsSuccessful()).To(BeTrue())
	})

	It("TestSetOrgRoleToOrgManager", func() {
		testSetOrgRoleWithValidRole("OrgManager", "/v2/organizations/my-org-guid/managers/my-user-guid")
	})

	It("TestSetOrgRoleToBillingManager", func() {
		testSetOrgRoleWithValidRole("BillingManager", "/v2/organizations/my-org-guid/billing_managers/my-user-guid")
	})

	It("TestSetOrgRoleToOrgAuditor", func() {
		testSetOrgRoleWithValidRole("OrgAuditor", "/v2/organizations/my-org-guid/auditors/my-user-guid")
	})

	It("TestSetOrgRoleWithInvalidRole", func() {
		repo := createUsersRepoWithoutEndpoints()
		apiResponse := repo.SetOrgRole("user-guid", "org-guid", "foo")

		Expect(apiResponse.IsSuccessful()).To(BeFalse())
		Expect(apiResponse.Message).To(ContainSubstring("Invalid Role"))
	})

	It("TestUnsetOrgRoleFromOrgManager", func() {
		testUnsetOrgRoleWithValidRole("OrgManager", "/v2/organizations/my-org-guid/managers/my-user-guid")
	})

	It("TestUnsetOrgRoleFromBillingManager", func() {
		testUnsetOrgRoleWithValidRole("BillingManager", "/v2/organizations/my-org-guid/billing_managers/my-user-guid")
	})

	It("TestUnsetOrgRoleFromOrgAuditor", func() {
		testUnsetOrgRoleWithValidRole("OrgAuditor", "/v2/organizations/my-org-guid/auditors/my-user-guid")
	})

	It("TestUnsetOrgRoleWithInvalidRole", func() {
		repo := createUsersRepoWithoutEndpoints()
		apiResponse := repo.UnsetOrgRole("user-guid", "org-guid", "foo")

		Expect(apiResponse.IsSuccessful()).To(BeFalse())
		Expect(apiResponse.Message).To(ContainSubstring("Invalid Role"))
	})

	It("TestSetSpaceRoleToSpaceManager", func() {
		testSetSpaceRoleWithValidRole("SpaceManager", "/v2/spaces/my-space-guid/managers/my-user-guid")
	})

	It("TestSetSpaceRoleToSpaceDeveloper", func() {
		testSetSpaceRoleWithValidRole("SpaceDeveloper", "/v2/spaces/my-space-guid/developers/my-user-guid")
	})

	It("TestSetSpaceRoleToSpaceAuditor", func() {
		testSetSpaceRoleWithValidRole("SpaceAuditor", "/v2/spaces/my-space-guid/auditors/my-user-guid")
	})

	It("TestSetSpaceRoleWithInvalidRole", func() {
		repo := createUsersRepoWithoutEndpoints()
		apiResponse := repo.SetSpaceRole("user-guid", "space-guid", "org-guid", "foo")

		Expect(apiResponse.IsSuccessful()).To(BeFalse())
		Expect(apiResponse.Message).To(ContainSubstring("Invalid Role"))
	})

	It("lists all users in the org", func() {
		ccReqs, uaaReqs := createUsersByRoleEndpoints("/v2/organizations/my-org-guid/users")

		cc, ccHandler, uaa, uaaHandler, repo := createUsersRepo(ccReqs, uaaReqs)
		defer cc.Close()
		defer uaa.Close()

		users, apiResponse := repo.ListUsersInOrgForRole("my-org-guid", models.ORG_USER)

		Expect(ccHandler.AllRequestsCalled()).To(BeTrue())
		Expect(uaaHandler.AllRequestsCalled()).To(BeTrue())
		Expect(apiResponse.IsSuccessful()).To(BeTrue())

		Expect(len(users)).To(Equal(3))
		Expect(users[0].Guid).To(Equal("user-1-guid"))
		Expect(users[0].Username).To(Equal("Super user 1"))
		Expect(users[1].Guid).To(Equal("user-2-guid"))
		Expect(users[1].Username).To(Equal("Super user 2"))
		Expect(users[2].Guid).To(Equal("user-3-guid"))
		Expect(users[2].Username).To(Equal("Super user 3"))
	})
})

func createUsersByRoleEndpoints(rolePath string) (ccReqs []testnet.TestRequest, uaaReqs []testnet.TestRequest) {
	nextUrl := rolePath + "?page=2"

	ccReqs = []testnet.TestRequest{
		testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method: "GET",
			Path:   rolePath,
			Response: testnet.TestResponse{
				Status: http.StatusOK,
				Body: fmt.Sprintf(`
				{
					"next_url": "%s",
					"resources": [
						{"metadata": {"guid": "user-1-guid"}, "entity": {}}
					]
				}`, nextUrl)}}),

		testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method: "GET",
			Path:   nextUrl,
			Response: testnet.TestResponse{
				Status: http.StatusOK,
				Body: `
				{
					"resources": [
					 	{"metadata": {"guid": "user-2-guid"}, "entity": {}},
					 	{"metadata": {"guid": "user-3-guid"}, "entity": {}}
					]
				}`}}),
	}

	uaaReqs = []testnet.TestRequest{
		testapi.NewCloudControllerTestRequest(testnet.TestRequest{
			Method: "GET",
			Path: fmt.Sprintf(
				"/Users?attributes=id,userName&filter=%s",
				url.QueryEscape(`Id eq "user-1-guid" or Id eq "user-2-guid" or Id eq "user-3-guid"`)),
			Response: testnet.TestResponse{
				Status: http.StatusOK,
				Body: `
				{
					"resources": [
						{ "id": "user-1-guid", "userName": "Super user 1" },
						{ "id": "user-2-guid", "userName": "Super user 2" },
  						{ "id": "user-3-guid", "userName": "Super user 3" }
					]
				}`}})}

	return
}

func testSetOrgRoleWithValidRole(role string, path string) {
	req := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
		Method:   "PUT",
		Path:     path,
		Response: testnet.TestResponse{Status: http.StatusOK},
	})

	userReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
		Method:   "PUT",
		Path:     "/v2/organizations/my-org-guid/users/my-user-guid",
		Response: testnet.TestResponse{Status: http.StatusOK},
	})

	cc, handler, repo := createUsersRepoWithoutUAAEndpoints([]testnet.TestRequest{req, userReq})
	defer cc.Close()

	apiResponse := repo.SetOrgRole("my-user-guid", "my-org-guid", role)

	Expect(handler.AllRequestsCalled()).To(BeTrue())
	Expect(apiResponse.IsSuccessful()).To(BeTrue())
}

func testUnsetOrgRoleWithValidRole(role string, path string) {
	req := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
		Method:   "DELETE",
		Path:     path,
		Response: testnet.TestResponse{Status: http.StatusOK},
	})

	cc, handler, repo := createUsersRepoWithoutUAAEndpoints([]testnet.TestRequest{req})
	defer cc.Close()

	apiResponse := repo.UnsetOrgRole("my-user-guid", "my-org-guid", role)

	Expect(handler.AllRequestsCalled()).To(BeTrue())
	Expect(apiResponse.IsSuccessful()).To(BeTrue())
}

func testSetSpaceRoleWithValidRole(role string, path string) {
	addToOrgReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
		Method:   "PUT",
		Path:     "/v2/organizations/my-org-guid/users/my-user-guid",
		Response: testnet.TestResponse{Status: http.StatusOK},
	})

	setRoleReq := testapi.NewCloudControllerTestRequest(testnet.TestRequest{
		Method:   "PUT",
		Path:     path,
		Response: testnet.TestResponse{Status: http.StatusOK},
	})

	cc, handler, repo := createUsersRepoWithoutUAAEndpoints([]testnet.TestRequest{addToOrgReq, setRoleReq})
	defer cc.Close()

	apiResponse := repo.SetSpaceRole("my-user-guid", "my-space-guid", "my-org-guid", role)

	Expect(handler.AllRequestsCalled()).To(BeTrue())
	Expect(apiResponse.IsSuccessful()).To(BeTrue())
}

func createUsersRepoWithoutEndpoints() (repo UserRepository) {
	_, _, _, _, repo = createUsersRepo([]testnet.TestRequest{}, []testnet.TestRequest{})
	return
}

func createUsersRepoWithoutUAAEndpoints(ccReqs []testnet.TestRequest) (cc *httptest.Server, ccHandler *testnet.TestHandler, repo UserRepository) {
	cc, ccHandler, _, _, repo = createUsersRepo(ccReqs, []testnet.TestRequest{})
	return
}

func createUsersRepoWithoutCCEndpoints(uaaReqs []testnet.TestRequest) (uaa *httptest.Server, uaaHandler *testnet.TestHandler, repo UserRepository) {
	_, _, uaa, uaaHandler, repo = createUsersRepo([]testnet.TestRequest{}, uaaReqs)
	return
}

func createUsersRepo(ccReqs []testnet.TestRequest, uaaReqs []testnet.TestRequest) (cc *httptest.Server,
	ccHandler *testnet.TestHandler, uaa *httptest.Server, uaaHandler *testnet.TestHandler, repo UserRepository) {

	ccTarget := ""
	uaaTarget := ""

	if len(ccReqs) > 0 {
		cc, ccHandler = testnet.NewTLSServer(ccReqs)
		ccTarget = cc.URL
	}
	if len(uaaReqs) > 0 {
		uaa, uaaHandler = testnet.NewTLSServer(uaaReqs)
		uaaTarget = uaa.URL
	}

	configRepo := testconfig.NewRepositoryWithDefaults()
	configRepo.SetApiEndpoint(ccTarget)
	ccGateway := net.NewCloudControllerGateway()
	uaaGateway := net.NewUAAGateway()
	endpointRepo := &testapi.FakeEndpointRepo{}
	endpointRepo.UAAEndpointReturns.Endpoint = uaaTarget
	repo = NewCloudControllerUserRepository(configRepo, uaaGateway, ccGateway, endpointRepo)
	return
}
