package extdns

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/labels"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	v1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

type Reconciler struct {
	Client       ctrlclient.Client
	Namespace    string
	SiteID       string
	DomainSuffix string
}

type Stats struct {
	Created int
	Updated int
	Deleted int
}

// Reconcile drives the desired DNSEndpoint set into the cluster: creating
// missing entries, updating drifted ones, and deleting those no longer
// desired. The desired map is consumed (entries are deleted as processed).
func (r *Reconciler) Reconcile(ctx context.Context, desired map[string]*v1alpha1.DNSEndpoint) (Stats, error) {
	var stats Stats

	list := v1alpha1.DNSEndpointList{}
	selector := labels.Set{SiteIDLabel: r.SiteID}.AsSelector()
	if err := r.Client.List(ctx, &list, ctrlclient.InNamespace(r.Namespace), ctrlclient.MatchingLabelsSelector{Selector: selector}); err != nil {
		return stats, fmt.Errorf("list existing: %w", err)
	}

	existing := make(map[string]*v1alpha1.DNSEndpoint)
	for i := range list.Items {
		item := list.Items[i]
		existing[item.Name] = &item
	}

	for name, obj := range existing {
		d, ok := desired[name]
		if !ok {
			if err := r.Client.Delete(ctx, obj); err != nil {
				return stats, fmt.Errorf("delete %s: %w", name, err)
			}
			stats.Deleted++
			continue
		}

		dLabels := d.GetLabels()
		dSpec := d.Spec

		if !reflect.DeepEqual(dSpec, obj.Spec) || !reflect.DeepEqual(dLabels, obj.GetLabels()) {
			obj.Spec = dSpec
			obj.SetLabels(dLabels)
			if err := r.Client.Update(ctx, obj); err != nil {
				return stats, fmt.Errorf("update %s: %w", name, err)
			}
			stats.Updated++
		}

		delete(desired, name)
	}

	for name, obj := range desired {
		if err := r.Client.Create(ctx, obj); err != nil {
			return stats, fmt.Errorf("create %s: %w", name, err)
		}
		stats.Created++
		delete(desired, name)
	}

	return stats, nil
}
