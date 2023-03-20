package caddy_gcp_secret_manager_test

import (
	"context"
	"flag"
	"fmt"
	caddy_gcp_secret_manager "github.com/trea/caddy-gcp-secret-manager"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"
)

var projectId string

func storage(t *testing.T) *caddy_gcp_secret_manager.SecretManagerStorage {
	storage, err := caddy_gcp_secret_manager.NewSecretManagerStorage(projectId)

	if err != nil {
		t.Fatalf("New failed (probably couldn't find credentials and init client): %+v", err)
	}

	return storage
}

func lockId() string {
	return fmt.Sprintf("test-lock-%d", time.Now().Unix())
}

func keyId() string {
	return fmt.Sprintf("test-key-%d", time.Now().Unix())
}

func TestUnlockBeforeLockFails(t *testing.T) {
	ctx := context.Background()

	storage := storage(t)

	err := storage.Unlock(ctx, lockId())

	if err == nil {
		t.Errorf("Unlocking before locking should fail")
	}
}

func TestAcquireLock(t *testing.T) {
	ctx := context.Background()

	storage := storage(t)

	err := storage.Lock(ctx, lockId())

	if err != nil {
		t.Fatalf("Lock should have been acquired, got err: %+v", err)
	}
}

func TestLockThenUnlock(t *testing.T) {
	ctx := context.Background()

	storage := storage(t)

	id := lockId()

	err := storage.Lock(ctx, id)

	if err != nil {
		t.Fatalf("Lock should have been acquired, got err: %+v", err)
	}

	err = storage.Unlock(ctx, id)

	if err != nil {
		t.Fatalf("Unlock should have worked, got err: %+v", err)
	}
}

func TestCantGetLockThatsOpen(t *testing.T) {
	ctx := context.Background()

	s1 := storage(t)
	s2 := storage(t)

	err := s1.Lock(ctx, "my-lock-exists")

	if err != nil {
		t.Fatalf("Should have been able to get lock")
	}

	err = s2.Lock(ctx, "my-lock-exists")

	if err == nil {
		t.Fatalf("Shouldn't have been able to acquire lock again")
	}
}

func TestStore(t *testing.T) {
	ctx := context.Background()

	s := storage(t)

	key := keyId()

	err := s.Store(ctx, key, []byte("some-value"))

	defer func() {
		if err := s.Delete(ctx, key); err != nil {
			t.Errorf("Delete after Store should work")
		}
	}()

	if err != nil {
		t.Fatalf("Store should have worked")
	}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	s := storage(t)
	s2 := storage(t)

	key := keyId()

	if err := s.Store(ctx, key, []byte("test")); err != nil {
		t.Errorf("Store should have worked")
	}

	t.Cleanup(func() {
		s.Delete(ctx, key)
	})

	if val, err := s2.Load(context.Background(), key); err != nil {
		t.Errorf("Load should have worked from the other instance")
	} else {
		if stringVal := string(val); stringVal != "test" {
			t.Errorf("Expected value to be 'test' saw '%s'", stringVal)
		}
	}
}

func TestExists(t *testing.T) {
	ctx := context.Background()

	s := storage(t)
	s2 := storage(t)

	key := keyId()

	if err := s.Store(ctx, key, []byte("test")); err != nil {
		t.Errorf("Store should have worked")
	}

	t.Cleanup(func() {
		s.Delete(ctx, key)
	})

	if exists := s2.Exists(context.Background(), key); !exists {
		t.Errorf("Exists should have found the key that was created")
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	s := storage(t)

	count := rand.Intn(20)

	if count == 0 {
		count += 1
	}

	unixNow := time.Now().Unix()

	prefix := fmt.Sprintf("list-test-key-%d", unixNow)

	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s-%d", prefix, i)
		if err := s.Store(ctx, key, []byte("test")); err != nil {
			t.Errorf("Store should have worked, got err: %+v", err)
		}

		t.Cleanup(func() {
			s.Delete(ctx, key)
		})
	}

	keys, err := s.List(ctx, prefix, false)

	if err != nil {
		t.Errorf("List should have worked, got err: %+v", err)
	}

	if len(keys) != count {
		t.Errorf("Expected List to return %d keys, got %d instead", count, len(keys))
	}

	for _, v := range keys {
		t.Logf("See key: %s", v)
	}
}

func TestStat(t *testing.T) {
	ctx := context.Background()
	s := storage(t)

	key := keyId()

	err := s.Store(ctx, key, []byte("stat-test"))

	if err != nil {
		t.Errorf("Store should have worked, got err: %+v", err)
	}

	info, err := s.Stat(ctx, key)

	t.Cleanup(func() {
		s.Delete(ctx, key)
	})

	if err != nil {
		t.Errorf("Stat should have worked, got err: %+v", err)
	}

	if info.Key != key {
		t.Errorf("Expected Stat to return key name '%s', got '%s' instead", key, info.Key)
	}

	if info.Modified.IsZero() {
		t.Errorf("Expected Modified to be set, is zero")
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("Delete should have worked, got err: %+v", err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	if val := os.Getenv("INTEGRATION_PROJECT_ID"); val == "" {
		log.Fatalf("Integration tests must be run with INTEGRATION_PROJECT_ID (and GOOGLE_APPLICATION_CREDENTIALS) set")
	} else {
		projectId = val
	}

	m.Run()
}
