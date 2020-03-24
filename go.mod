module github.com/kubemove/elasticsearch-plugin

go 1.13

require (
	github.com/appscode/go v0.0.0-20200225060711-86360b91102a
	github.com/elastic/cloud-on-k8s v0.0.0-20200227085127-963e594a6b97
	github.com/elastic/go-elasticsearch/v7 v7.6.0
	github.com/go-logr/logr v0.1.0
	github.com/kubemove/kubemove v0.0.0-20200311123929-e3139a5e30bd
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.5
	google.golang.org/grpc v1.27.1
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.17.3
	sigs.k8s.io/controller-runtime v0.4.0
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	github.com/kubemove/kubemove => github.com/hossainemruz/kubemove v0.0.0-20200324065950-4a3cf00edc98
)
