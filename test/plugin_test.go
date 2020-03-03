package test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/kubemove/elasticsearch-plugin/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var opt util.PluginOptions

func TestMain(m *testing.M) {
	flag.StringVar(&opt.KubeConfigPath, "kubeconfig", "", "KubeConfig path")
	flag.StringVar(&opt.SrcContext, "src-context", "", "Source Context")
	flag.StringVar(&opt.DstContext, "dst-context", "", "Destination Context")
	flag.StringVar(&opt.SrcPluginAddress, "src-plugin", "", "URL of the source plugin")
	flag.StringVar(&opt.DstPluginAddress, "dst-plugin", "", "URL of the destination plugin")
	flag.StringVar(&opt.SrcClusterIp, "src-cluster-ip", "", "IP address of the source cluster")
	flag.StringVar(&opt.DstClusterIp, "dst-cluster-ip", "", "IP address of the source cluster")
	flag.Int64Var(&opt.SrcESNodePort, "src-es-nodeport", 0, "Node port of source ES service")
	flag.Int64Var(&opt.DstESNodePort, "dst-es-nodeport", 0, "Node port of source ES service")
	flag.Parse()

	os.Exit(m.Run())
}
func TestEckPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elasticsearch Plugin Suite")
}

var _ = Describe("Elasticsearch Plugin Test", func() {

	BeforeEach(func() {
		err := opt.Setup()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Default Nodes", func() {
		It("should sync ES data between clusters", func() {
			By("Triggering INIT API to prepare for sync")
			err := opt.TriggerInit()
			Expect(err).NotTo(HaveOccurred())

			By("Creating a sample index in the source ES")
			opt.IndexName = "e2e_demo"
			err = opt.InsertIndex()
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that Index has been inserted successfully in the source ES")
			opt.IndexFrom = "active"
			resp, err := opt.ShowIndexes()
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Contains(resp, opt.IndexName)).Should(BeTrue())

			By("Triggering SYNC API to sync data between the ES clusters")
			err = opt.TriggerSync()
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that index has been synced in the destination ES")
			opt.IndexFrom = "standby"
			resp, err = opt.ShowIndexes()
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Contains(resp, opt.IndexName)).Should(BeTrue())
		})
	})
})
