package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

var _ = Describe("reconcileEndpoints with envtest", Ordered, func() {
	var (
		testEnv   *envtest.Environment
		k8sClient ctrlclient.Client
		ctx       context.Context
		cancel    context.CancelFunc
	)

	BeforeAll(func() {
		ctx, cancel = context.WithCancel(context.Background())

		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		testEnv = &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				CRDs: []*apiextensionsv1.CustomResourceDefinition{externalDNSEndpointCRD()},
			},
		}

		var err error
		cfg, err := testEnv.Start()
		if err != nil {
			// if binaries missing, skip
			Skip("envtest binaries not available: " + err.Error())
		}

		k8sClient, err = ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
		Expect(err).ToNot(HaveOccurred())

		// ensure namespace exists
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})).To(Succeed())
	})

	AfterAll(func() {
		cancel()
		if testEnv != nil {
			_ = testEnv.Stop()
		}
	})

	It("creates, updates, and deletes endpoints", func() {
		cfg := Config{Namespace: "default", SiteID: "site", DomainSuffix: "example.com"}

		desired := desiredResources(cfg, []client{{Name: "host1", IPAddress: "10.0.0.1"}})
		stats, err := reconcileEndpoints(ctx, k8sClient, cfg, desired)
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Created).To(Equal(1))

		// wait until created is observed via list
		eventuallyList := func() ([]v1alpha1.DNSEndpoint, error) {
			var list v1alpha1.DNSEndpointList
			err := k8sClient.List(ctx, &list)
			return list.Items, err
		}
		Eventually(func() int {
			items, err := eventuallyList()
			if err != nil {
				return -1
			}
			return len(items)
		}, 5*time.Second, 200*time.Millisecond).Should(Equal(1))

		// update desired target
		desired = desiredResources(cfg, []client{{Name: "host1", IPAddress: "10.0.0.2"}})
		stats, err = reconcileEndpoints(ctx, k8sClient, cfg, desired)
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Updated).To(Equal(1))

		Eventually(func() string {
			var obj v1alpha1.DNSEndpoint
			err := k8sClient.Get(ctx, ctrlclient.ObjectKey{Name: "host1", Namespace: "default"}, &obj)
			if err != nil {
				return ""
			}
			if len(obj.Spec.Endpoints) == 0 {
				return ""
			}
			return obj.Spec.Endpoints[0].Targets[0]
		}, 5*time.Second, 200*time.Millisecond).Should(Equal("10.0.0.2"))

		// delete
		desired = map[string]*v1alpha1.DNSEndpoint{}
		stats, err = reconcileEndpoints(ctx, k8sClient, cfg, desired)
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Deleted).To(Equal(1))

		Eventually(func() bool {
			var list v1alpha1.DNSEndpointList
			_ = k8sClient.List(ctx, &list)
			return len(list.Items) == 0
		}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())
	})
})

func externalDNSEndpointCRD() *apiextensionsv1.CustomResourceDefinition {
	plural := "dnsendpoints"
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: plural + ".externaldns.k8s.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "externaldns.k8s.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     plural,
				Singular:   "dnsendpoint",
				Kind:       "DNSEndpoint",
				ShortNames: []string{"dnsendpoint", "dnsendpoints"},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1alpha1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"spec": {
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"endpoints": {
										Type: "array",
										Items: &apiextensionsv1.JSONSchemaPropsOrArray{
											Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
										},
									},
								},
							},
						},
					},
				},
				Subresources: &apiextensionsv1.CustomResourceSubresources{Status: &apiextensionsv1.CustomResourceSubresourceStatus{}},
			}},
		},
	}

	return crd
}
