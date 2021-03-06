package requirements_test

import (
	"cf/models"
	. "cf/requirements"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testapi "testhelpers/api"
	testassert "testhelpers/assert"
	testterm "testhelpers/terminal"
)

var _ = Describe("Testing with ginkgo", func() {
	It("TestBuildpackReqExecute", func() {

		buildpack := models.Buildpack{}
		buildpack.Name = "my-buildpack"
		buildpack.Guid = "my-buildpack-guid"
		buildpackRepo := &testapi.FakeBuildpackRepository{FindByNameBuildpack: buildpack}
		ui := new(testterm.FakeUI)

		buildpackReq := NewBuildpackRequirement("foo", ui, buildpackRepo)
		success := buildpackReq.Execute()

		Expect(success).To(BeTrue())
		Expect(buildpackRepo.FindByNameName).To(Equal("foo"))
		Expect(buildpackReq.GetBuildpack()).To(Equal(buildpack))
	})
	It("TestBuildpackReqExecuteWhenBuildpackNotFound", func() {

		buildpackRepo := &testapi.FakeBuildpackRepository{FindByNameNotFound: true}
		ui := new(testterm.FakeUI)

		buildpackReq := NewBuildpackRequirement("foo", ui, buildpackRepo)

		testassert.AssertPanic(testterm.FailedWasCalled, func() {
			buildpackReq.Execute()
		})
	})
})
