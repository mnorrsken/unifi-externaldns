package extdns_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

var testScheme *runtime.Scheme

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "extdns Suite")
}

var _ = BeforeSuite(func() {
	testScheme = runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(testScheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(testScheme)).To(Succeed())
})

func newFakeClient() ctrlclient.Client {
	return fake.NewClientBuilder().WithScheme(testScheme).Build()
}
