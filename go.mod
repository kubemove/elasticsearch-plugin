module github.com/kubemove/elasticsearch-plugin

go 1.13

require (
	github.com/appscode/go v0.0.0-20200225060711-86360b91102a
	github.com/elastic/cloud-on-k8s v0.0.0-20200227085127-963e594a6b97
	github.com/elastic/go-elasticsearch/v7 v7.6.0
	github.com/go-logr/logr v0.1.0
	github.com/kubemove/kubemove v0.0.0-00010101000000-000000000000
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	google.golang.org/grpc v1.27.1
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.4.0
)

replace github.com/kubemove/kubemove => github.com/hossainemruz/kubemove v0.0.0-20200227111208-9a0c2567596e
