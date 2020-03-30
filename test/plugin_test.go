package test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/appscode/go/crypto/rand"

	"github.com/kubemove/elasticsearch-plugin/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var opt util.PluginOptions
var i Invocation

func TestMain(m *testing.M) {
	flag.StringVar(&opt.KubeConfigPath, "kubeconfigpath", "", "KubeConfig path")
	flag.StringVar(&opt.SrcContext, "src-context", "", "Source Context")
	flag.StringVar(&opt.DstContext, "dst-context", "", "Destination Context")
	flag.StringVar(&opt.SrcPluginAddress, "src-plugin", "", "URL of the source plugin")
	flag.StringVar(&opt.DstPluginAddress, "dst-plugin", "", "URL of the destination plugin")
	flag.StringVar(&opt.SrcClusterIp, "src-cluster-ip", "", "IP address of the source cluster")
	flag.StringVar(&opt.DstClusterIp, "dst-cluster-ip", "", "IP address of the source cluster")
	flag.Parse()

	os.Exit(m.Run())
}

var _ = BeforeSuite(func() {
	By("Preparing clients")
	err := opt.Setup()
	Expect(err).NotTo(HaveOccurred())

	By("Deploying a Minio Server")
	err = deployMinioServer(&opt)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("Removing Minio Server")
	err := removeMinioServer(&opt)
	Expect(err).NotTo(HaveOccurred())
})

func TestEckPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elasticsearch Plugin Suite")
}

var _ = Describe("Elasticsearch Plugin Test", func() {
	BeforeEach(func() {
		i = NewInvocation(&opt)
	})
	Context("Minio Repository", func() {
		BeforeEach(func() {
			By("Creating bucket: " + i.testID)
			err := i.createMinioBucket()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Default Nodes", func() {
			It("should sync ES data between clusters", func() {
				By("Creating sample Elasticsearch")
				es := newDefaultElasticsearch()
				err := i.createElasticsearch(es)
				Expect(err).NotTo(HaveOccurred())

				By("Creating a sample index in the source ES")
				opt.IndexName = rand.WithUniqSuffix("e2e-demo")
				err = opt.InsertIndex()
				Expect(err).NotTo(HaveOccurred())

				By("Verifying that Index has been inserted successfully in the source ES")
				opt.IndexFrom = "active"
				resp, err := opt.ShowIndexes()
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.Contains(resp, opt.IndexName)).Should(BeTrue())

				By("Creating MoveEngine CR in the source cluster")
				engine, err := i.newSampleMoveEngine(es)
				Expect(err).NotTo(HaveOccurred())
				err = i.createMoveEngine(engine)
				Expect(err).NotTo(HaveOccurred())

				By("Verifying that standby MoveEngine has been created in the destination cluster")
				i.EventuallyStandbyMoveEngineCreated(engine.ObjectMeta).Should(BeTrue())

				By("Waiting for MoveEngine to be ready")
				i.EventuallyMoveEngineReady(engine.ObjectMeta).Should(BeTrue())

				By("Waiting for a DataSync CR in the source cluster")
				i.EventuallySourceDataSyncCreated(engine.ObjectMeta).Should(BeTrue())

				By("Waiting for a DataSync CR in the destination cluster")
				i.EventuallyDestinationDataSyncCreated(engine.ObjectMeta).Should(BeTrue())

				By("Waiting for the Sync to be completed")
				i.EventuallySyncSucceeded(engine.ObjectMeta).Should(BeTrue())

				By("Verifying that index has been synced in the destination ES")
				opt.IndexFrom = "standby"
				resp, err = opt.ShowIndexes()
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.Contains(resp, opt.IndexName)).Should(BeTrue())
			})
		})
	})
})
