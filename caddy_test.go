package caddy_gcp_secret_manager_test

import (
	"context"
	"errors"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	caddy_gcp_secret_manager "github.com/trea/caddy-gcp-secret-manager"
	"testing"
	"testing/fstest"
)

func TestUnmarshalBadCaddyfile(t *testing.T) {
	cf := `gcp-secret-manager`

	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{}
	d := caddyfile.NewTestDispenser(cf)

	err := s.UnmarshalCaddyfile(d)

	if err == nil {
		t.Fatalf("Unmarshaling Caddyfile should have failed since there's no project ID")
	}
}

func TestUnmarshalBasicCaddyfile(t *testing.T) {
	cf := `gcp-secret-manager my-project-id`

	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{}
	d := caddyfile.NewTestDispenser(cf)

	err := s.UnmarshalCaddyfile(d)

	if err != nil {
		t.Fatalf("Unmarshal should have worked, got err: %+v", err)
	}

	if s.ProjectID != "my-project-id" {
		t.Fatalf("Expected ProjectID to be '%s', is '%s'", "my-project-id", s.ProjectID)
	}
}

func TestUnmarshalCaddyfileWithCredentialsFile(t *testing.T) {
	cf := `
gcp-secret-manager test-project-id {
	credentials_file /app/gcp-credentials.json
}`

	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{}
	d := caddyfile.NewTestDispenser(cf)

	err := s.UnmarshalCaddyfile(d)

	if err != nil {
		t.Fatalf("Unmarshal should have worked, got err: %+v", err)
	}

	if s.ProjectID != "test-project-id" {
		t.Errorf("Expected ProjectID to be '%s', got '%s' instead", "test-project-id", s.ProjectID)
	}

	if s.CredentialsFile != "/app/gcp-credentials.json" {
		t.Errorf("Expected CredentialsFile to be '%s', got '%s' instead", "/app/gcp-credentials.json", s.CredentialsFile)
	}
}

func TestUnmarshalCaddyfileWithProjectAndCredentialsNested(t *testing.T) {
	cf := `
gcp-secret-manager {
	project_id test-project-id
	credentials_file /app/gcp-credentials.json
}`

	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{}
	d := caddyfile.NewTestDispenser(cf)

	err := s.UnmarshalCaddyfile(d)

	if err != nil {
		t.Fatalf("Unmarshal should have worked, got err: %+v", err)
	}

	if s.ProjectID != "test-project-id" {
		t.Errorf("Expected ProjectID to be '%s', got '%s' instead", "test-project-id", s.ProjectID)
	}

	if s.CredentialsFile != "/app/gcp-credentials.json" {
		t.Errorf("Expected CredentialsFile to be '%s', got '%s' instead", "/app/gcp-credentials.json", s.CredentialsFile)
	}
}

func TestProvision(t *testing.T) {
	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{}

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	if err := s.Provision(ctx); err != nil {
		t.Fatalf("Expected provision to pass, got err: %+v", err)
	}
}

func TestValidationFailsWhenGivenCredentialsFileThatDoesntExist(t *testing.T) {
	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{
		CredentialsFile: "some-garbage-credentials-file.json",
	}

	if err := s.Validate(); err == nil {
		t.Fatalf("Expected Validate to fail because the file didn't exist")
	} else if errors.Is(err, caddy_gcp_secret_manager.ErrCredentialsFile) == false {
		t.Errorf("Expected error to be ErrCredentialsFile")
	}
}

func TestValidationFailsWhenGivenCredentialsFileThatExistsButContainsInvalidJson(t *testing.T) {
	fs := fstest.MapFS{
		"credentials.json": {
			Data: []byte("{this-isnt-json}"),
		},
	}

	s := caddy_gcp_secret_manager.CaddyGcpSecretManagerStorage{
		CredentialsFile: "credentials.json",
		Fs:              fs,
	}

	if err := s.Validate(); err == nil {
		t.Fatalf("Expected Validate to fail because the file contains invalid json")
	} else if errors.Is(err, caddy_gcp_secret_manager.ErrInvalidJSON) == false {
		t.Errorf("Expected error returned to be ErrInvalidJSON")
	}
}
