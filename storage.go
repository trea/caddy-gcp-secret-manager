package caddy_gcp_secret_manager

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"errors"
	"fmt"
	"github.com/caddyserver/certmagic"
	"github.com/sethvargo/go-retry"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/durationpb"
	"net/url"
	"strings"
	"time"
)

type SecretManagerStorage struct {
	projectID string
	client    *secretmanager.Client
	etags     map[string]string
	locks     map[string]bool
}

func NewSecretManagerStorage(projectID string, opts ...option.ClientOption) (*SecretManagerStorage, error) {
	client, err := secretmanager.NewClient(context.Background(), opts...)

	if err != nil {
		return nil, fmt.Errorf("unable to setup GCP Client: %w", err)
	}

	return &SecretManagerStorage{
		projectID,
		client,
		make(map[string]string),
		make(map[string]bool),
	}, nil
}

func (s SecretManagerStorage) getParent() string {
	return fmt.Sprintf("projects/%s", s.projectID)
}

func (s SecretManagerStorage) Close() error {
	return s.client.Close()
}

func (s SecretManagerStorage) store(ctx context.Context, name string, content []byte, ttl time.Duration) error {
	createRequest := &secretmanagerpb.CreateSecretRequest{
		Parent:   s.getParent(),
		SecretId: name,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{Automatic: &secretmanagerpb.Replication_Automatic{}},
			},
		},
	}

	if ttl != 0 {
		createRequest.Secret.Expiration = &secretmanagerpb.Secret_Ttl{
			Ttl: durationpb.New(1 * time.Minute),
		}
	}

	secret, err := s.client.CreateSecret(ctx, createRequest)

	if err != nil {
		return err
	}

	version, err := s.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: content,
		},
	})

	if err != nil {
		return err
	}

	s.etags[name] = version.Etag

	return nil
}

var ErrKeyNotFound = errors.New("unable to find key")

func (s *SecretManagerStorage) findKey(ctx context.Context, name string) (*secretmanagerpb.Secret, error) {
	list := s.client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: s.getParent(),
		Filter: fmt.Sprintf("name:%s", name),
	})

	var secret *secretmanagerpb.Secret
	var err error
	for {
		secret, err = list.Next()

		if err == iterator.Done {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("unable to lookup key: %+v", err)
		}

		if strings.HasSuffix(secret.Name, name) {
			break
		}
	}

	if secret == nil {
		return nil, ErrKeyNotFound
	}

	s.etags[name] = secret.Etag

	return secret, nil
}

func (s SecretManagerStorage) readLatest(ctx context.Context, secret *secretmanagerpb.Secret) (*secretmanagerpb.SecretVersion, error) {
	versionPath, err := url.JoinPath(secret.Name, "versions", "latest")
	if err != nil {
		return nil, err
	}

	version, err := s.client.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
		Name: versionPath,
	})

	return version, err
}

func (s *SecretManagerStorage) Lock(ctx context.Context, name string) error {
	name = fmt.Sprintf("lock-%s", name)

	if exists := s.locks[name]; exists {
		return fmt.Errorf("lock already exists")
	}

	secret, err := s.findKey(ctx, name)

	if err != nil && errors.Is(err, ErrKeyNotFound) == false {
		return err
	}

	if secret != nil {
		return fmt.Errorf("lock already exists")
	}

	return retry.Fibonacci(ctx, 1*time.Second, func(ctx context.Context) error {
		err = s.store(ctx, name, []byte("true"), 1*time.Minute)

		if err != nil {
			return err
		}

		s.locks[name] = true

		return nil
	})
}

func (s *SecretManagerStorage) Unlock(ctx context.Context, name string) error {
	name = fmt.Sprintf("lock-%s", name)

	if exists := s.locks[name]; !exists {
		return fmt.Errorf("called Unlock before Lock: '%s' '%+v", name, s.locks)
	}

	return retry.Fibonacci(ctx, 1*time.Second, func(ctx context.Context) error {
		secret, err := s.findKey(ctx, name)

		if err != nil {
			return err
		}

		if secret == nil {
			return fmt.Errorf("unable to locate lock: %+v", err)
		}

		err = s.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
			Name: secret.Name,
			Etag: s.etags[name],
		})

		if err != nil {
			return fmt.Errorf("unable to clear lock: %+v", err)
		}

		delete(s.locks, name)
		return nil
	})
}

func (s SecretManagerStorage) Store(ctx context.Context, key string, value []byte) error {
	return s.store(ctx, key, value, 0)
}

func (s SecretManagerStorage) Load(ctx context.Context, key string) ([]byte, error) {
	secret, err := s.findKey(ctx, key)

	if err != nil {
		return nil, err
	}

	latest, err := s.readLatest(ctx, secret)

	if err != nil {
		return nil, err
	}

	version, err := s.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: latest.Name,
	})

	if err != nil {
		return nil, err
	}

	return version.Payload.Data, nil
}

func (s SecretManagerStorage) Delete(ctx context.Context, key string) error {
	secret, err := s.findKey(ctx, key)

	if err != nil {
		return err
	}

	if secret == nil {
		return fmt.Errorf("unable to locate key: %+v", err)
	}

	return s.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: secret.Name,
		Etag: s.etags[key],
	})
}

func (s SecretManagerStorage) Exists(ctx context.Context, key string) bool {
	secret, _ := s.findKey(ctx, key)

	return secret != nil
}

func (s SecretManagerStorage) List(ctx context.Context, prefix string, recursive bool) (list []string, err error) {
	it := s.client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: s.getParent(),
		Filter: prefix,
	})

	for {
		secret, err := it.Next()

		if err == iterator.Done {
			break
		}

		if err != nil {
			return list, err
		}

		parts := strings.Split(secret.Name, "/")

		key := parts[len(parts)-1]

		list = append(list, key)
	}

	return list, nil
}

func (s SecretManagerStorage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	secret, err := s.findKey(ctx, key)

	if err != nil {
		return certmagic.KeyInfo{}, err
	}

	latest, err := s.readLatest(ctx, secret)

	if err != nil {
		return certmagic.KeyInfo{}, err
	}

	parts := strings.Split(secret.Name, "/")

	trimmedKey := parts[len(parts)-1]

	return certmagic.KeyInfo{
		Key:        trimmedKey,
		Modified:   latest.CreateTime.AsTime(),
		IsTerminal: true,
	}, nil
}

var _ certmagic.Storage = (*SecretManagerStorage)(nil)
