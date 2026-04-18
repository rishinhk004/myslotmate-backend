package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pineconesdk "github.com/pinecone-io/go-pinecone/v4/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

// PineconeClient handles Pinecone vector database operations through the official Go SDK.
type PineconeClient struct {
	apiKey    string
	host      string
	indexName string

	mu      sync.Mutex
	client  *pineconesdk.Client
	index   *pineconesdk.IndexConnection
	initErr error
}

// NewPineconeClient creates a new Pinecone client.
func NewPineconeClient(apiKey, host, indexName string) *PineconeClient {
	return &PineconeClient{
		apiKey:    strings.TrimSpace(apiKey),
		host:      sanitizePineconeHost(host),
		indexName: strings.TrimSpace(indexName),
	}
}

// Query searches Pinecone for similar vectors.
func (p *PineconeClient) Query(ctx context.Context, queryVector []float32, topK int32, filter map[string]interface{}) ([]map[string]interface{}, error) {
	idx, err := p.ensureIndex(ctx)
	if err != nil {
		return nil, err
	}

	metadataFilter, err := toMetadataStruct(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to build query filter: %w", err)
	}

	resp, err := idx.QueryByVectorValues(ctx, &pineconesdk.QueryByVectorValuesRequest{
		Vector:          queryVector,
		TopK:            uint32(topK),
		MetadataFilter:  metadataFilter,
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query Pinecone: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(resp.Matches))
	for _, match := range resp.Matches {
		result := map[string]interface{}{
			"score": match.Score,
		}
		if match.Vector != nil {
			result["id"] = match.Vector.Id
			if match.Vector.Metadata != nil {
				result["metadata"] = match.Vector.Metadata.AsMap()
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// Upsert stores or updates vectors in Pinecone.
func (p *PineconeClient) Upsert(ctx context.Context, vectors []map[string]interface{}) error {
	if len(vectors) == 0 {
		return nil
	}

	idx, err := p.ensureIndex(ctx)
	if err != nil {
		return err
	}

	sdkVectors := make([]*pineconesdk.Vector, 0, len(vectors))
	for i, vector := range vectors {
		id, _ := vector["id"].(string)
		values, ok := vector["values"].([]float32)
		if !ok {
			return fmt.Errorf("vector %d has invalid values payload", i)
		}

		metadata, err := metadataFromAny(vector["metadata"])
		if err != nil {
			return fmt.Errorf("vector %d has invalid metadata payload: %w", i, err)
		}

		sdkVector := &pineconesdk.Vector{
			Id:     id,
			Values: &values,
		}
		if metadata != nil {
			sdkVector.Metadata = metadata
		}

		sdkVectors = append(sdkVectors, sdkVector)
	}

	if _, err := idx.UpsertVectors(ctx, sdkVectors); err != nil {
		return fmt.Errorf("failed to upsert to Pinecone: %w", err)
	}

	return nil
}

// Delete removes vectors from Pinecone.
func (p *PineconeClient) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	idx, err := p.ensureIndex(ctx)
	if err != nil {
		return err
	}

	if err := idx.DeleteVectorsById(ctx, ids); err != nil {
		return fmt.Errorf("failed to delete from Pinecone: %w", err)
	}

	return nil
}

func (p *PineconeClient) ensureIndex(ctx context.Context) (*pineconesdk.IndexConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.index != nil {
		return p.index, nil
	}
	if p.initErr != nil {
		return nil, p.initErr
	}
	if p.apiKey == "" {
		p.initErr = fmt.Errorf("Pinecone API key is not configured")
		return nil, p.initErr
	}

	client, err := pineconesdk.NewClient(pineconesdk.NewClientParams{
		ApiKey: p.apiKey,
	})
	if err != nil {
		p.initErr = fmt.Errorf("failed to create Pinecone client: %w", err)
		return nil, p.initErr
	}

	host := p.host
	if host == "" {
		if p.indexName == "" {
			p.initErr = fmt.Errorf("Pinecone host is not configured and no index name is available to resolve it")
			return nil, p.initErr
		}

		indexInfo, err := client.DescribeIndex(ctx, p.indexName)
		if err != nil {
			p.initErr = fmt.Errorf("failed to resolve Pinecone host for index %q: %w", p.indexName, err)
			return nil, p.initErr
		}
		host = sanitizePineconeHost(indexInfo.Host)
		p.host = host
	}

	index, err := client.Index(pineconesdk.NewIndexConnParams{
		Host: host,
	})
	if err != nil {
		p.initErr = fmt.Errorf("failed to create Pinecone index connection for host %q: %w", host, err)
		return nil, p.initErr
	}

	p.client = client
	p.index = index
	return p.index, nil
}

func sanitizePineconeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	return host
}

func toMetadataStruct(filter map[string]interface{}) (*pineconesdk.MetadataFilter, error) {
	if len(filter) == 0 {
		return nil, nil
	}
	return structpb.NewStruct(filter)
}

func metadataFromAny(value interface{}) (*pineconesdk.Metadata, error) {
	if value == nil {
		return nil, nil
	}

	switch metadata := value.(type) {
	case map[string]interface{}:
		return structpb.NewStruct(metadata)
	case map[string]string:
		converted := make(map[string]interface{}, len(metadata))
		for key, value := range metadata {
			converted[key] = value
		}
		return structpb.NewStruct(converted)
	default:
		return nil, fmt.Errorf("unsupported metadata type %T", value)
	}
}
