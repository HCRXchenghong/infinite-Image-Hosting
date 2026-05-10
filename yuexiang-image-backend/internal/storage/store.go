package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	ContentType string
	Private     bool
}

type ObjectStore interface {
	Ping(ctx context.Context) error
	PutObject(ctx context.Context, input PutObjectInput) error
	GetObject(ctx context.Context, key string) ([]byte, string, error)
	DeleteObject(ctx context.Context, key string) error
}

type MemoryObjectStore struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

type memoryObject struct {
	contentType string
	body        []byte
}

func NewMemoryObjectStore() *MemoryObjectStore {
	return &MemoryObjectStore{objects: map[string]memoryObject{}}
}

func (s *MemoryObjectStore) Ping(_ context.Context) error {
	return nil
}

func (s *MemoryObjectStore) PutObject(_ context.Context, input PutObjectInput) error {
	if input.Key == "" {
		return fmt.Errorf("object key is required")
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[input.Key] = memoryObject{contentType: input.ContentType, body: body}
	return nil
}

func (s *MemoryObjectStore) GetObject(_ context.Context, key string) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.objects[key]
	if !ok {
		return nil, "", fmt.Errorf("object not found")
	}
	return bytes.Clone(obj.body), obj.contentType, nil
}

func (s *MemoryObjectStore) DeleteObject(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

type S3CompatibleConfig struct {
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	ForcePathStyle bool   `json:"force_path_style"`
	AccessKey      string `json:"-"`
	SecretKey      string `json:"-"`
}
