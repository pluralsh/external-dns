package plural

import (
	"os"
	"fmt"
	"context"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
	log "github.com/sirupsen/logrus"
)

const (
	CreateAction = "c"
	DeleteAction = "d"
)

type PluralProvider struct {
	provider.BaseProvider
	Client *Client
}

type RecordChange struct {
	Action string
	Record *DnsRecord
}

func NewPluralProvider(cluster, provider string) (*PluralProvider, error) {
	token := os.Getenv("PLURAL_ACCESS_TOKEN")
	endpoint := os.Getenv("PLURAL_ENDPOINT")

	if token == "" {
		return nil, fmt.Errorf("No plural access token provided, you must set the PLURAL_ACCESS_TOKEN env var")
	}

	config := &Config{
		Token: token,
		Endpoint: endpoint,
		Cluster: cluster,
		Provider: provider,
	}
	prov := &PluralProvider{
		Client: NewClient(config),
	}

	return prov, nil
}

func (p *PluralProvider) Records(ctx context.Context) (endpoints []*endpoint.Endpoint, err error) {
	records, err := p.Client.DnsRecords()
	if err != nil {
		return
	}

	endpoints = make([]*endpoint.Endpoint, len(records))
	for i, record := range records {
		endpoints[i] = endpoint.NewEndpoint(record.Name, record.Type, record.Records...)
	}
	return
}

func (p *PluralProvider) PropertyValuesEqual(name, previous, current string) bool {
	return p.BaseProvider.PropertyValuesEqual(name, previous, current)
}

func (p *PluralProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) []*endpoint.Endpoint {
	return endpoints
}

func (p *PluralProvider) ApplyChanges(ctx context.Context, diffs *plan.Changes) error {
	changes := []*RecordChange{}
	for _, endpoint := range diffs.Create {
		changes = append(changes, makeChange(CreateAction, endpoint.Targets, endpoint))
	}

	for _, desired := range diffs.UpdateNew {
		changes = append(changes, makeChange(CreateAction, desired.Targets, desired))
	}

	for _, deleted := range diffs.Delete {
		changes = append(changes, makeChange(DeleteAction, []string{}, deleted))
	}

	return p.applyChanges(changes)
}

func makeChange(change string, target []string, endpoint *endpoint.Endpoint) *RecordChange {
	return &RecordChange{
		Action: change,
		Record: &DnsRecord{
			Name: endpoint.DNSName,
			Type: endpoint.RecordType,
			Records: target,
		},
	}
}

func (p *PluralProvider) applyChanges(changes []*RecordChange) error {
	for _, change := range changes {
		logFields := log.Fields{
			"name":   change.Record.Name,
			"type":   change.Record.Type,
			"action": change.Action,
		}
		log.WithFields(logFields).Info("Changing record.")

		if change.Action == CreateAction {
			_, err := p.Client.CreateRecord(change.Record)
			if err != nil {
				return err
			}
		}
		if change.Action == DeleteAction {
			if err := p.Client.DeleteRecord(change.Record.Name, change.Record.Type); err != nil {
				return err
			}
		}
	}

	return nil
}